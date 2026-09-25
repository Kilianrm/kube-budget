package wails

import (
	"fmt"
	"strings"
	"time"

	"kube-budget/core/costmodel"
	"kube-budget/core/costseries"
	"kube-budget/core/optimize"
	costexplorermode "kube-budget/internal/application/costexplorer"
	"kube-budget/internal/providers"
	"kube-budget/internal/storage/costhistory"
)

// CostReportResult pairs the cost report with the recommendations derived
// from it, so both views of the Cost section share one calculation.
type CostReportResult struct {
	Report          costmodel.CostReport      `json:"report"`
	Recommendations []optimize.Recommendation `json:"recommendations"`
	ClusterID       string                    `json:"clusterId"`
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

	providerName := pricingProviderName(request)
	selectedProvider, ok := providers.Get(providerName)
	if !ok {
		return CostReportResult{}, fmt.Errorf("cost report: unsupported pricing provider %q; supported providers: %s", providerName, strings.Join(providers.Names(), ", "))
	}

	region := strings.TrimSpace(request.PricingRegion)
	if region == "" {
		region = strings.TrimSpace(request.Region)
	}

	report, err := costexplorermode.New(selectedProvider).Report(snapshot, costexplorermode.Options{
		Provider:   selectedProvider.Name(),
		Region:     selectedProvider.NormalizeRegion(region),
		PricingSKU: strings.TrimSpace(request.InstanceType),
	})
	if err != nil {
		return CostReportResult{}, err
	}

	result := CostReportResult{
		Report:          report,
		Recommendations: optimize.Analyze(report),
		ClusterID:       costhistory.ClusterID(report.Scope),
	}
	if err := recordCostHistory(report); err != nil {
		result.Report.Warnings = append(result.Report.Warnings, costmodel.Warning{
			Code:    "history-not-recorded",
			Message: err.Error(),
		})
	}
	return result, nil
}

// GetCostTrend integrates the recorded captures into spend over time. Periods
// without captures are returned as gaps, never interpolated.
func (adapter *ClusterAdapter) GetCostTrend(request CostTrendRequest) (costseries.Series, error) {
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
		return costseries.Series{}, err
	}

	to := time.Now().UTC()
	from := to.AddDate(0, 0, -days)
	records, err := store.Range(strings.TrimSpace(request.ClusterID), from, to)
	if err != nil {
		return costseries.Series{}, err
	}

	samples := make([]costseries.Sample, 0, len(records))
	currency := costmodel.CurrencyUSD
	for _, record := range records {
		if record.Currency != "" {
			currency = record.Currency
		}
		samples = append(samples, costseries.Sample{
			At:                record.CapturedAt,
			ProvisionedHourly: record.HourlyByBasis[string(costmodel.BasisProvisioned)],
			RequestedHourly:   record.HourlyByBasis[string(costmodel.BasisRequested)],
			IdleHourly:        record.IdleHourly,
		})
	}

	series := costseries.Build(samples, from, to, bucket, costseries.DefaultMaxGap)
	series.Currency = currency
	return series, nil
}

func recordCostHistory(report costmodel.CostReport) error {
	store, err := defaultCostHistoryStore()
	if err != nil {
		return err
	}
	return store.Append(costhistory.FromReport(report, time.Now()))
}

func defaultCostHistoryStore() (*costhistory.Store, error) {
	path, err := costhistory.DefaultPath()
	if err != nil {
		return nil, err
	}
	return costhistory.New(path), nil
}

// pricingProviderName maps a cluster provider such as "aws-eks" onto the
// catalog that prices it.
func pricingProviderName(request CostReportRequest) string {
	if name := strings.TrimSpace(request.PricingProvider); name != "" {
		return strings.ToLower(name)
	}
	name := strings.ToLower(strings.TrimSpace(request.Provider))
	if index := strings.Index(name, "-"); index > 0 {
		name = name[:index]
	}
	return name
}
