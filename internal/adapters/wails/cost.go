package wails

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"kube-budget/core/costmodel"
	"kube-budget/core/costseries"
	clustermode "kube-budget/internal/application/cluster"
	costexplorermode "kube-budget/internal/application/costexplorer"
	"kube-budget/internal/providers"
	"kube-budget/internal/storage/budget"
	"kube-budget/internal/storage/costhistory"
	"kube-budget/internal/storage/recommendations"
)

// CostReportResult pairs the cost report with the recommendations derived
// from it, so both views of the Cost section share one calculation.
type CostReportResult struct {
	Report       costmodel.CostReport `json:"report"`
	Optimization OptimizationResult   `json:"optimization"`
	ClusterID    string               `json:"clusterId"`
}

// CostTrendResult is the recorded spend over a window plus the totals of the
// window of equal length just before it, so the UI can show a change.
type CostTrendResult struct {
	Series   costseries.Series  `json:"series"`
	Previous costseries.Summary `json:"previous"`
	// Drivers explains the change in rate against the previous window.
	Drivers []costseries.Driver `json:"drivers"`
}

// CostForecastRequest asks for the current month's forecast. RunRateHourly is
// the billed rate of the report on screen, so both views use one number.
type CostForecastRequest struct {
	ClusterID     string  `json:"clusterId"`
	RunRateHourly float64 `json:"runRateHourly"`
}

// CostForecastResult is the month's forecast and, when set, its budget.
type CostForecastResult struct {
	Forecast costseries.Forecast      `json:"forecast"`
	Budget   *costseries.BudgetStatus `json:"budget,omitempty"`
}

// CostBudgetRequest sets a cluster's monthly budget; zero removes it.
type CostBudgetRequest struct {
	ClusterID  string  `json:"clusterId"`
	MonthlyUSD float64 `json:"monthlyUSD"`
}

// CostTrendRequest asks for recorded spend over a past window.
type CostTrendRequest struct {
	ClusterID string `json:"clusterId"`
	Days      int    `json:"days"`
	Bucket    string `json:"bucket"`
}

// CostReportRequest collects a cluster snapshot and prices it in one call.
type CostReportRequest struct {
	KubeconfigPath string `json:"kubeconfigPath"`
	Context        string `json:"context"`
	Namespace      string `json:"namespace"`
	Provider       string `json:"provider"`
	ClusterName    string `json:"clusterName"`
	Region         string `json:"region"`
	Profile        string `json:"profile"`
	RoleARN        string `json:"roleArn"`
	// PricingProvider selects the price catalog. It defaults to the catalog
	// implied by Provider.
	PricingProvider string `json:"pricingProvider"`
	PricingRegion   string `json:"pricingRegion"`
	InstanceType    string `json:"instanceType"`
}

// GetCostReport returns the cluster cost report rendered by the Cost Explorer.
func (adapter *ClusterAdapter) GetCostReport(request CostReportRequest) (CostReportResult, error) {
	snapshot, err := adapter.GetClusterSnapshot(ClusterSnapshotRequest{
		KubeconfigPath: request.KubeconfigPath,
		Context:        request.Context,
		Namespace:      request.Namespace,
		Provider:       request.Provider,
		ClusterName:    request.ClusterName,
		Region:         request.Region,
		Profile:        request.Profile,
		RoleARN:        request.RoleARN,
	})
	if err != nil {
		return CostReportResult{}, err
	}

	// An explicit pricing provider wins; otherwise the cluster's platform,
	// detected from its nodes, picks the price list.
	providerName := strings.ToLower(strings.TrimSpace(request.PricingProvider))
	region := strings.TrimSpace(request.PricingRegion)
	if providerName == "" {
		if !snapshot.Platform.Supported {
			return CostReportResult{}, fmt.Errorf("cost report: this cluster cannot be priced: %s", snapshot.Platform.Reason)
		}
		providerName = snapshot.Platform.Provider
	}
	if region == "" {
		region = snapshot.Platform.Region
	}
	selectedProvider, ok := providers.Get(providerName)
	if !ok {
		return CostReportResult{}, fmt.Errorf("cost report: unsupported pricing provider %q; supported providers: %s", providerName, strings.Join(providers.Names(), ", "))
	}

	report, err := costexplorermode.New(selectedProvider).Report(snapshot, costexplorermode.Options{
		Provider:   selectedProvider.Name(),
		Region:     selectedProvider.NormalizeRegion(region),
		PricingSKU: strings.TrimSpace(request.InstanceType),
		Namespace:  collectionNamespace(request.Namespace),
	})
	if err != nil {
		return CostReportResult{}, err
	}

	clusterID := costhistory.ClusterID(report.Scope)
	inputs := planInputs(selectedProvider, report.Scope.Region)
	inputs.EKS = eksCluster(snapshot.Provider)
	inputs.NodeTransition = nodeTransition(snapshot.Provider)
	adapter.rememberCost(clusterID, report, inputs)

	result := CostReportResult{Report: report, ClusterID: clusterID}
	store, err := defaultRecommendationStore()
	if err == nil {
		result.Optimization, err = optimization(store, clusterID, report, inputs)
	}
	if err != nil {
		// The plan still works without the user's dismissals and journal.
		result.Optimization = optimizationFrom(recommendations.State{}, report, inputs)
		result.Report.Warnings = append(result.Report.Warnings, costmodel.Warning{Code: "recommendations-unavailable", Message: err.Error()})
	}
	if !adapter.captureDue(clusterID, report, time.Now()) {
		return result, nil
	}
	if err := recordCostHistory(report); err != nil {
		// Not written, so the next report tries again.
		adapter.captureMutex.Lock()
		delete(adapter.lastCaptures, clusterID)
		adapter.captureMutex.Unlock()
		result.Report.Warnings = append(result.Report.Warnings, costmodel.Warning{
			Code:    "history-not-recorded",
			Message: err.Error(),
		})
	}
	return result, nil
}

// GetCostTrend integrates the recorded captures into spend over time. Periods
// without captures are returned as gaps, never interpolated.
func (adapter *ClusterAdapter) GetCostTrend(request CostTrendRequest) (CostTrendResult, error) {
	days := request.Days
	if days <= 0 {
		days = 7
	}
	bucket := costseries.BucketHour
	if strings.EqualFold(request.Bucket, string(costseries.BucketDay)) {
		bucket = costseries.BucketDay
	}

	store, err := defaultCostHistoryStore()
	if err != nil {
		return CostTrendResult{}, err
	}

	// Local time, so day buckets follow the user's calendar.
	to := time.Now()
	from := to.AddDate(0, 0, -days)
	previousFrom := from.AddDate(0, 0, -days)
	records, err := store.Range(strings.TrimSpace(request.ClusterID), previousFrom, to)
	if err != nil {
		return CostTrendResult{}, err
	}

	samples := make([]costseries.Sample, 0, len(records))
	currency := costmodel.CurrencyUSD
	for _, record := range records {
		if record.Currency != "" {
			currency = record.Currency
		}
		samples = append(samples, sampleFromRecord(record))
	}

	series := costseries.Build(samples, from, to, bucket, costseries.DefaultMaxGap)
	series.Currency = currency
	previous := costseries.Build(samples, previousFrom, from, bucket, costseries.DefaultMaxGap)
	return CostTrendResult{Series: series, Previous: previous.Summary(), Drivers: costseries.Drivers(series, previous)}, nil
}

// GetCostForecast projects the current month's spend and compares it with the
// cluster's budget.
func (adapter *ClusterAdapter) GetCostForecast(request CostForecastRequest) (CostForecastResult, error) {
	inputs, err := loadForecastInputs(strings.TrimSpace(request.ClusterID), time.Now())
	if err != nil {
		return CostForecastResult{}, err
	}
	return inputs.forecast(request.RunRateHourly), nil
}

// forecastInputs is the recorded month and the budget of one cluster, loaded
// once so several run rates can be forecast against them.
type forecastInputs struct {
	records       []costhistory.Record
	now           time.Time
	monthlyBudget float64
}

func loadForecastInputs(clusterID string, now time.Time) (forecastInputs, error) {
	history, err := defaultCostHistoryStore()
	if err != nil {
		return forecastInputs{}, err
	}
	budgets, err := defaultBudgetStore()
	if err != nil {
		return forecastInputs{}, err
	}

	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	// A capture just before the month still covers its first hour.
	records, err := history.Range(clusterID, monthStart.Add(-costseries.DefaultMaxGap), now)
	if err != nil {
		return forecastInputs{}, err
	}
	stored, ok, err := budgets.Get(clusterID)
	if err != nil {
		return forecastInputs{}, err
	}
	inputs := forecastInputs{records: records, now: now}
	if ok {
		inputs.monthlyBudget = stored.MonthlyUSD
	}
	return inputs, nil
}

func (inputs forecastInputs) forecast(runRateHourly float64) CostForecastResult {
	return forecastResult(inputs.records, inputs.now, runRateHourly, inputs.monthlyBudget)
}

// withRunRateChange re-forecasts after a change that starts now, with the
// budget evaluated against the new total.
func (inputs forecastInputs) withRunRateChange(before CostForecastResult, deltaHourly float64) CostForecastResult {
	after := CostForecastResult{Forecast: before.Forecast.WithRunRateChange(deltaHourly)}
	if inputs.monthlyBudget > 0 {
		status := costseries.EvaluateBudget(after.Forecast, inputs.monthlyBudget)
		after.Budget = &status
	}
	return after
}

// SetCostBudget stores or removes a cluster's monthly budget.
func (adapter *ClusterAdapter) SetCostBudget(request CostBudgetRequest) error {
	budgets, err := defaultBudgetStore()
	if err != nil {
		return err
	}
	return budgets.Set(strings.TrimSpace(request.ClusterID), request.MonthlyUSD, time.Now())
}

func forecastResult(records []costhistory.Record, now time.Time, runRateHourly, monthlyBudget float64) CostForecastResult {
	samples := make([]costseries.Sample, 0, len(records))
	for _, record := range records {
		samples = append(samples, sampleFromRecord(record))
	}
	result := CostForecastResult{Forecast: costseries.BuildForecast(samples, now, runRateHourly, costseries.DefaultMaxGap)}
	if monthlyBudget > 0 {
		status := costseries.EvaluateBudget(result.Forecast, monthlyBudget)
		result.Budget = &status
	}
	return result
}

func sampleFromRecord(record costhistory.Record) costseries.Sample {
	return costseries.Sample{
		At:                record.CapturedAt,
		ProvisionedHourly: record.HourlyByBasis[string(costmodel.BasisProvisioned)],
		RequestedHourly:   record.HourlyByBasis[string(costmodel.BasisRequested)],
		IdleHourly:        record.IdleHourly,
		SharedHourly:      record.SharedHourly,
		HourlyByNamespace: record.HourlyByNamespace,
		HourlyByNodeGroup: record.HourlyByNodeGroup,
	}
}

// historyCaptureSpacing is the least time between two captures of the same
// cluster at the same rate. Reports refresh as often as every 10 seconds; the
// history integrates rates over time, so repeating an unchanged rate adds
// nothing but size, and the 20,000-capture store would last only days.
const historyCaptureSpacing = 5 * time.Minute

// captureDue reports whether a report should be written to the history: the
// first one for its cluster, one whose billed rate changed, or one
// historyCaptureSpacing after the last capture.
func (adapter *ClusterAdapter) captureDue(clusterID string, report costmodel.CostReport, now time.Time) bool {
	adapter.captureMutex.Lock()
	defer adapter.captureMutex.Unlock()
	if adapter.lastCaptures == nil {
		adapter.lastCaptures = make(map[string]lastCapture)
	}
	rate := report.Totals[costmodel.BasisProvisioned].Hourly
	last, ok := adapter.lastCaptures[clusterID]
	if ok && now.Sub(last.at) < historyCaptureSpacing && math.Abs(rate-last.rate) < 1e-9 {
		return false
	}
	adapter.lastCaptures[clusterID] = lastCapture{at: now, rate: rate}
	return true
}

func recordCostHistory(report costmodel.CostReport) error {
	store, err := defaultCostHistoryStore()
	if err != nil {
		return err
	}
	return store.Append(costhistory.FromReport(report, time.Now()))
}

func defaultBudgetStore() (*budget.Store, error) {
	path, err := budget.DefaultPath()
	if err != nil {
		return nil, err
	}
	return budget.New(path), nil
}

func defaultCostHistoryStore() (*costhistory.Store, error) {
	path, err := costhistory.DefaultPath()
	if err != nil {
		return nil, err
	}
	return costhistory.New(path), nil
}

// pricingProviderName maps a cluster provider such as "aws-eks" onto the
// pricing catalog of its cloud ("aws").
func pricingProviderName(provider string) string {
	name := strings.ToLower(strings.TrimSpace(provider))
	if index := strings.Index(name, "-"); index > 0 {
		name = name[:index]
	}
	return name
}

var providerTitles = map[string]string{"aws": "AWS", "gcp": "Google Cloud", "azure": "Azure"}

// clusterPlatform settles which price list a cluster uses. The nodes decide;
// an EKS connection stands in for nodes that report nothing (no nodes yet).
// It is supported when that provider's catalog covers the region.
func clusterPlatform(nodes []clustermode.Node, connectionProvider, connectionRegion string) clustermode.Platform {
	platform := clustermode.DetectPlatform(nodes)
	if platform.Detected == "" {
		if name := pricingProviderName(connectionProvider); name != "" {
			platform.Provider, platform.Detected = name, name
		}
	}
	if platform.Region == "" {
		platform.Region = strings.TrimSpace(connectionRegion)
	}

	selected, ok := providers.Get(platform.Provider)
	switch {
	case platform.Detected == "":
		platform.Reason = "its nodes do not report a cloud provider, so it looks like a local or on-premises cluster, and those have no public price list"
	case !ok:
		platform.Reason = fmt.Sprintf("its nodes report %q, which has no public price list; costs can be calculated for clusters on %s", platform.Detected, supportedClouds())
	case platform.Region == "":
		platform.Reason = fmt.Sprintf("it runs on %s but its nodes carry no region label, so the regional prices cannot be chosen", providerTitles[platform.Provider])
	case !slices.Contains(selected.Regions(), selected.NormalizeRegion(platform.Region)):
		platform.Reason = fmt.Sprintf("it runs on %s in %s, a region the price catalog does not cover yet (covered: %s)", providerTitles[platform.Provider], platform.Region, strings.Join(selected.Regions(), ", "))
	default:
		platform.Region = selected.NormalizeRegion(platform.Region)
		platform.Supported = true
	}
	return platform
}

func supportedClouds() string {
	names := make([]string, 0, len(providerTitles))
	for _, name := range providers.Names() {
		if title, ok := providerTitles[name]; ok {
			names = append(names, title)
		}
	}
	return strings.Join(names, ", ")
}
