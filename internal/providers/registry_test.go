package providers

import "testing"

func TestNamesReturnsRegisteredProviders(t *testing.T) {
	names := Names()
	if len(names) != 3 || names[0] != "aws" || names[1] != "azure" || names[2] != "gcp" {
		t.Fatalf("Names() = %#v, want [aws azure gcp]", names)
	}
}

func TestGetReturnsUsableProviderContract(t *testing.T) {
	for _, test := range []struct {
		name         string
		region       string
		machineType  string
		wantProvider string
	}{
		{name: "aws", region: "us-east-1", machineType: "m6i.large", wantProvider: "aws"},
		{name: "azure", region: "eastus", machineType: "Standard_D2s_v5", wantProvider: "azure"},
		{name: "gcp", region: "us-central1", machineType: "e2-standard-2", wantProvider: "gcp"},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider, ok := Get(test.name)
			if !ok {
				t.Fatalf("Get(%q) did not find a provider", test.name)
			}
			if provider.Name() != test.wantProvider {
				t.Errorf("Name() = %q, want %q", provider.Name(), test.wantProvider)
			}
			if len(provider.Regions()) == 0 {
				t.Errorf("Regions() is empty")
			}
			if len(provider.MachineTypes(test.region)) == 0 {
				t.Errorf("MachineTypes(%q) is empty", test.region)
			}
			if _, err := provider.NewPriceConfig(test.machineType, test.region); err != nil {
				t.Errorf("NewPriceConfig() error = %v", err)
			}
		})
	}
}

func TestGetNormalizesProviderName(t *testing.T) {
	if provider, ok := Get(" GCP "); !ok || provider.Name() != "gcp" {
		t.Fatalf("Get() did not normalize provider name")
	}
}

func TestGetRejectsUnknownProvider(t *testing.T) {
	if _, ok := Get("oracle"); ok {
		t.Fatalf("Get(oracle) found an unregistered provider")
	}
}
