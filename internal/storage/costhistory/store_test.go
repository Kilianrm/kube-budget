package costhistory

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kube-budget/core/costmodel"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return New(filepath.Join(t.TempDir(), "history", "cost-history.ndjson"))
}

func sampleReport() costmodel.CostReport {
	items := []costmodel.LineItem{
		{Subject: costmodel.Subject{Kind: costmodel.SubjectNode, Name: "node-1", ParentID: "ng-1"}, Basis: costmodel.BasisProvisioned, HourlyUSD: 0.2, Confidence: costmodel.ConfidenceExact},
		{Subject: costmodel.Subject{Kind: costmodel.SubjectWorkload, Name: "api", Namespace: "prod"}, Basis: costmodel.BasisRequested, HourlyUSD: 0.05, Confidence: costmodel.ConfidenceDerived},
		{Subject: costmodel.Subject{Kind: costmodel.SubjectWorkload, Name: "legacy", Namespace: "prod"}, Basis: costmodel.BasisRequested, Confidence: costmodel.ConfidenceUnknown},
	}
	return costmodel.NewReport(time.Unix(0, 0), costmodel.Scope{Provider: "aws", Region: "us-east-1", ClusterName: "demo"}, items, nil)
}

func TestFromReportKeepsHourlyRatesOnly(t *testing.T) {
	record := FromReport(sampleReport(), time.Unix(3600, 0))

	if record.ClusterID != "aws/us-east-1/demo" {
		t.Errorf("ClusterID = %q, want aws/us-east-1/demo", record.ClusterID)
	}
	if record.HourlyByBasis["provisioned"] != 0.2 || record.HourlyByBasis["requested"] != 0.05 {
		t.Errorf("HourlyByBasis = %+v, want the two bases", record.HourlyByBasis)
	}
	if math.Abs(record.IdleHourly-0.15) > 1e-9 {
		t.Errorf("IdleHourly = %v, want 0.15", record.IdleHourly)
	}
	if record.NodeCount != 1 || record.WorkloadCount != 2 || record.Unpriced != 1 {
		t.Errorf("counts = %d nodes, %d workloads, %d unpriced", record.NodeCount, record.WorkloadCount, record.Unpriced)
	}
	if record.HourlyByNamespace["prod"] != 0.05 || record.HourlyByNodeGroup["ng-1"] != 0.2 {
		t.Errorf("rollups = %+v / %+v", record.HourlyByNamespace, record.HourlyByNodeGroup)
	}
}

func TestAppendAndRangeFilterByClusterAndWindow(t *testing.T) {
	store := newTestStore(t)
	base := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)

	for index := 0; index < 3; index++ {
		record := FromReport(sampleReport(), base.Add(time.Duration(index)*time.Hour))
		if err := store.Append(record); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}
	other := FromReport(sampleReport(), base)
	other.ClusterID = "aws/eu-west-1/other"
	if err := store.Append(other); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	records, err := store.Range("aws/us-east-1/demo", base, base.Add(90*time.Minute))
	if err != nil {
		t.Fatalf("Range() error = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("Range() returned %d records, want 2", len(records))
	}
	if !records[0].CapturedAt.Before(records[1].CapturedAt) {
		t.Error("Range() must return records oldest first")
	}

	clusters, err := store.Clusters()
	if err != nil {
		t.Fatalf("Clusters() error = %v", err)
	}
	if len(clusters) != 2 {
		t.Errorf("Clusters() = %v, want both clusters", clusters)
	}
}

func TestRangeOnMissingStoreIsEmpty(t *testing.T) {
	records, err := newTestStore(t).Range("any", time.Unix(0, 0), time.Now())
	if err != nil {
		t.Fatalf("Range() error = %v", err)
	}
	if len(records) != 0 {
		t.Errorf("Range() = %v, want no records", records)
	}
}

// One unreadable line must not hide the rest of the history.
func TestReadSkipsCorruptAndForeignVersionLines(t *testing.T) {
	store := newTestStore(t)
	if err := store.Append(FromReport(sampleReport(), time.Unix(0, 0))); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	file, err := os.OpenFile(store.path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, err := file.WriteString("{not json\n{\"schemaVersion\":999,\"clusterId\":\"aws/us-east-1/demo\"}\n"); err != nil {
		t.Fatalf("write store: %v", err)
	}
	file.Close()

	records, err := store.Range("", time.Unix(0, 0), time.Now())
	if err != nil {
		t.Fatalf("Range() error = %v", err)
	}
	if len(records) != 1 {
		t.Errorf("Range() = %d records, want only the readable one", len(records))
	}
}

func TestStoreFileIsPrivateToTheUser(t *testing.T) {
	store := newTestStore(t)
	if err := store.Append(FromReport(sampleReport(), time.Unix(0, 0))); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	info, err := os.Stat(store.path)
	if err != nil {
		t.Fatalf("stat store: %v", err)
	}
	if permissions := info.Mode().Perm(); permissions != 0o600 {
		t.Errorf("store permissions = %o, want 600", permissions)
	}
}

func TestAppendTrimsToTheRecordLimit(t *testing.T) {
	store := newTestStore(t)
	store.maxRecords = 3
	base := time.Unix(0, 0)

	for index := 0; index < 6; index++ {
		if err := store.Append(FromReport(sampleReport(), base.Add(time.Duration(index)*time.Hour))); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	records, err := store.Range("", base, base.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("Range() error = %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("Range() = %d records, want the 3 newest", len(records))
	}
	if !records[0].CapturedAt.Equal(base.Add(3 * time.Hour).UTC()) {
		t.Errorf("oldest kept record = %v, want the fourth capture", records[0].CapturedAt)
	}
}
