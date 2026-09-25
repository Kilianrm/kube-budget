package wails

import "testing"

func TestPricingProviderNameDerivesCatalogFromClusterProvider(t *testing.T) {
	for _, test := range []struct {
		name    string
		request CostReportRequest
		want    string
	}{
		{name: "explicit", request: CostReportRequest{Provider: "aws-eks", PricingProvider: "GCP"}, want: "gcp"},
		{name: "derived", request: CostReportRequest{Provider: "aws-eks"}, want: "aws"},
		{name: "plain", request: CostReportRequest{Provider: "Azure"}, want: "azure"},
		{name: "empty", request: CostReportRequest{}, want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := pricingProviderName(test.request); got != test.want {
				t.Errorf("pricingProviderName() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestGetCostReportRejectsUnknownPricingProvider(t *testing.T) {
	_, err := NewClusterAdapter().GetCostReport(CostReportRequest{Context: "", PricingProvider: "ibm"})
	if err == nil {
		t.Fatal("GetCostReport() error = nil, want an error")
	}
}
