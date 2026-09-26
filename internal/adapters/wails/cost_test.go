package wails

import (
	"math"
	"strings"
	"testing"
	"time"

	"kube-budget/core/costmodel"
	"kube-budget/core/costseries"
	"kube-budget/core/optimize"
	clustermode "kube-budget/internal/application/cluster"
	"kube-budget/internal/providers"
	"kube-budget/internal/storage/costhistory"
	"kube-budget/internal/storage/recommendations"
)

func TestPricingProviderNameDerivesCatalogFromClusterProvider(t *testing.T) {
	for provider, want := range map[string]string{"aws-eks": "aws", "Azure": "azure", "": ""} {
		if got := pricingProviderName(provider); got != want {
			t.Errorf("pricingProviderName(%q) = %q, want %q", provider, got, want)
		}
	}
}

func TestClusterPlatformDecidesWhetherTheCostSectionCanPriceACluster(t *testing.T) {
	eks := []clustermode.Node{{ProviderID: "aws:///us-east-1a/i-0abc", Region: "us-east-1"}}
	for _, test := range []struct {
		name       string
		nodes      []clustermode.Node
		connection string
		region     string
		supported  bool
		reason     string
	}{
		{name: "eks through a kubeconfig context", nodes: eks, supported: true},
		{name: "eks connection without nodes yet", connection: "aws-eks", region: "eu-west-1", supported: true},
		{name: "kind", nodes: []clustermode.Node{{ProviderID: "kind://docker/kind/kind-control-plane"}}, reason: `"kind"`},
		{name: "minikube", nodes: []clustermode.Node{{Name: "minikube"}}, reason: "local or on-premises"},
		{name: "region outside the catalog", nodes: []clustermode.Node{{ProviderID: "aws:///ap-south-1a/i-0abc", Region: "ap-south-1"}}, reason: "ap-south-1"},
		{name: "no region", nodes: []clustermode.Node{{ProviderID: "aws:///zone/i-0abc"}}, reason: "no region label"},
	} {
		t.Run(test.name, func(t *testing.T) {
			platform := clusterPlatform(test.nodes, test.connection, test.region)
			if platform.Supported != test.supported || !strings.Contains(platform.Reason, test.reason) {
				t.Errorf("platform = %+v, want supported %v with a reason mentioning %q", platform, test.supported, test.reason)
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

func TestForecastResultAttachesTheBudgetOnlyWhenSet(t *testing.T) {
	now := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	records := []costhistory.Record{{
		CapturedAt:        now.Add(-30 * time.Minute),
		HourlyByBasis:     map[string]float64{"provisioned": 2},
		HourlyByNodeGroup: map[string]float64{"general": 2},
	}}

	withoutBudget := forecastResult(records, now, 2, 0)
	if withoutBudget.Budget != nil {
		t.Errorf("Budget = %+v, want nil without a budget", withoutBudget.Budget)
	}
	// 30 recorded minutes, 359.5 estimated hours and 360 projected hours, all at 2/h
	if got := withoutBudget.Forecast.TotalUSD; got < 1439.99 || got > 1440.01 {
		t.Errorf("TotalUSD = %v, want 1440", got)
	}

	withBudget := forecastResult(records, now, 2, 1000)
	if withBudget.Budget == nil || withBudget.Budget.State != costseries.BudgetOver {
		t.Errorf("Budget = %+v, want over", withBudget.Budget)
	}
}

func TestOptimizationPricesTheAppliedJournal(t *testing.T) {
	report := costmodel.NewReport(time.Unix(0, 0), costmodel.Scope{}, []costmodel.LineItem{{
		Subject: costmodel.Subject{Kind: costmodel.SubjectNode, Name: "a"}, Basis: costmodel.BasisProvisioned,
		HourlyUSD: 1.0, Confidence: costmodel.ConfidenceExact,
	}}, nil)
	state := recommendations.State{
		Dismissed: map[string]time.Time{"spot-capacity": time.Unix(0, 0)},
		Applied:   []recommendations.Applied{{ID: "consolidate-nodes", BaselineHourly: 1.2, ExpectedHourly: 0.25}},
	}

	result := optimizationFrom(state, report, optimize.Inputs{})

	if len(result.Applied) != 1 || math.Abs(result.Applied[0].Realized.Hourly-0.2) > 1e-9 || result.Applied[0].Expected.Hourly != 0.25 {
		t.Errorf("Applied = %+v, want 0.2/h realized against 0.25/h expected", result.Applied)
	}
}

func TestPlanInputsFromTheAWSCatalog(t *testing.T) {
	provider, _ := providers.Get("aws")

	inputs := planInputs(provider, "us-east-1")

	if len(inputs.MachineTypes) == 0 || math.Abs(inputs.SpotPriceRatio-0.35) > 1e-9 {
		t.Fatalf("inputs = %+v, want the catalog's machine types and spot ratio", inputs)
	}
	for _, machine := range inputs.MachineTypes {
		if machine.Burstable != (machine.Name == "t3.medium") {
			t.Errorf("%s burstable = %v", machine.Name, machine.Burstable)
		}
	}
}
