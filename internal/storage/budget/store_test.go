package budget

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSetGetAndRemove(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "nested", "budgets.json"))
	at := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)

	if _, ok, err := store.Get("aws/us-east-1/demo"); err != nil || ok {
		t.Fatalf("Get() on an empty store = %v, %v, want no budget", ok, err)
	}
	if err := store.Set("aws/us-east-1/demo", 1200, at); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	budget, ok, err := store.Get("aws/us-east-1/demo")
	if err != nil || !ok || budget.MonthlyUSD != 1200 || !budget.UpdatedAt.Equal(at) {
		t.Fatalf("Get() = %+v, %v, %v, want 1200", budget, ok, err)
	}
	if err := store.Set("aws/us-east-1/demo", 0, at); err != nil {
		t.Fatalf("Set(0) error = %v", err)
	}
	if _, ok, _ := store.Get("aws/us-east-1/demo"); ok {
		t.Error("budget still set after Set(0), want it removed")
	}
}

func TestSetRequiresACluster(t *testing.T) {
	if err := New(filepath.Join(t.TempDir(), "budgets.json")).Set("", 10, time.Now()); err == nil {
		t.Error("Set() with an empty cluster ID, want an error")
	}
}
