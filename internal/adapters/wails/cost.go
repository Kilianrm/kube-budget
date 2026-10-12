package wails

import (
	"math"
	"strings"
	"time"

	"kube-cost-workspace/core/costmodel"
	"kube-cost-workspace/core/costseries"
	"kube-cost-workspace/internal/application/workspace"
	"kube-cost-workspace/internal/storage/recommendations"
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

	priced, err := workspace.Price(snapshot, workspace.PriceOptions{
		Provider:     request.PricingProvider,
		Region:       request.PricingRegion,
		InstanceType: request.InstanceType,
		Namespace:    request.Namespace,
	})
	if err != nil {
		return CostReportResult{}, err
	}
	report, clusterID := priced.Report, priced.ClusterID
	adapter.rememberCost(clusterID, report, priced.Plan)

	result := CostReportResult{Report: report, ClusterID: clusterID}
	store, err := workspace.RecommendationStore()
	if err == nil {
		result.Optimization, err = optimization(store, clusterID, report, priced.Plan)
	}
	if err != nil {
		// The plan still works without the user's dismissals and journal.
		result.Optimization = optimizationFrom(recommendations.State{}, report, priced.Plan)
		result.Report.Warnings = append(result.Report.Warnings, costmodel.Warning{Code: "recommendations-unavailable", Message: err.Error()})
	}
	if !adapter.captureDue(clusterID, report, time.Now()) {
		return result, nil
	}
	if err := workspace.RecordHistory(report, time.Now()); err != nil {
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
	bucket := costseries.BucketHour
	if strings.EqualFold(request.Bucket, string(costseries.BucketDay)) {
		bucket = costseries.BucketDay
	}
	// Local time, so day buckets follow the user's calendar.
	trend, err := workspace.SpendTrend(request.ClusterID, request.Days, bucket, time.Now())
	if err != nil {
		return CostTrendResult{}, err
	}
	return CostTrendResult{Series: trend.Series, Previous: trend.Previous, Drivers: trend.Drivers}, nil
}

// GetCostForecast projects the current month's spend and compares it with the
// cluster's budget.
func (adapter *ClusterAdapter) GetCostForecast(request CostForecastRequest) (CostForecastResult, error) {
	inputs, err := workspace.LoadForecast(request.ClusterID, time.Now())
	if err != nil {
		return CostForecastResult{}, err
	}
	return forecastResult(inputs.Forecast(request.RunRateHourly)), nil
}

// SetCostBudget stores or removes a cluster's monthly budget.
func (adapter *ClusterAdapter) SetCostBudget(request CostBudgetRequest) error {
	return workspace.SetBudget(request.ClusterID, request.MonthlyUSD, time.Now())
}

func forecastResult(forecast workspace.Forecast) CostForecastResult {
	return CostForecastResult{Forecast: forecast.Forecast, Budget: forecast.Budget}
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
