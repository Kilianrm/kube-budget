package gcp

import (
	"testing"
)

func TestNewPriceConfigForRegion(t *testing.T) {
	config, err := NewPriceConfigForRegion("e2-standard-2", "us-central1")
	if err != nil {
		t.Fatalf("NewPriceConfigForRegion() error = %v", err)
	}

	if config.CPUUSDPerCore != 0.01675 {
		t.Errorf("CPUUSDPerCore = %v, want 0.01675", config.CPUUSDPerCore)
	}
	if config.MemoryUSDPerGB != 0.0041875 {
		t.Errorf("MemoryUSDPerGB = %v, want 0.0041875", config.MemoryUSDPerGB)
	}
	if config.StorageUSDPerGB != 0.04/730 {
		t.Errorf("StorageUSDPerGB = %v, want %v", config.StorageUSDPerGB, 0.04/730)
	}
}

func TestNewPriceConfigForRegionRejectsUnknownValues(t *testing.T) {
	for _, test := range []struct {
		name          string
		machineType   string
		region        string
		wantErrorText string
	}{
		{name: "region", machineType: "e2-standard-2", region: "us-east1", wantErrorText: "unsupported region"},
		{name: "machine type", machineType: "e2-medium", region: "us-central1", wantErrorText: "unknown machine type"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewPriceConfigForRegion(test.machineType, test.region)
			if err == nil || !contains(err.Error(), test.wantErrorText) {
				t.Fatalf("NewPriceConfigForRegion() error = %v, want %q", err, test.wantErrorText)
			}
		})
	}
}

func contains(value, substring string) bool {
	for index := 0; index+len(substring) <= len(value); index++ {
		if value[index:index+len(substring)] == substring {
			return true
		}
	}
	return false
}
