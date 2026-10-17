package azure

import "testing"

func TestNewPriceConfigForRegion(t *testing.T) {
	config, err := NewPriceConfigForRegion("Standard_D2s_v5", "eastus")
	if err != nil {
		t.Fatalf("NewPriceConfigForRegion() error = %v", err)
	}

	if config.CPUUSDPerCore != 0.024 {
		t.Errorf("CPUUSDPerCore = %v, want 0.024", config.CPUUSDPerCore)
	}
	if config.MemoryUSDPerGB != 0.006 {
		t.Errorf("MemoryUSDPerGB = %v, want 0.006", config.MemoryUSDPerGB)
	}
	wantStorage := float64(0.05) / float64(730)
	if config.StorageUSDPerGB != wantStorage {
		t.Errorf("StorageUSDPerGB = %v, want %v", config.StorageUSDPerGB, wantStorage)
	}
}

func TestNewPriceConfigForRegionRejectsUnknownValues(t *testing.T) {
	for _, test := range []struct {
		name          string
		vmSize        string
		region        string
		wantErrorText string
	}{
		{name: "region", vmSize: "Standard_D2s_v5", region: "centralus", wantErrorText: "unsupported region"},
		{name: "VM size", vmSize: "Standard_B2s", region: "eastus", wantErrorText: "unknown VM size"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewPriceConfigForRegion(test.vmSize, test.region)
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
