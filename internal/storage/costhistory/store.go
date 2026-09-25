// Package costhistory persists compact cost captures so the Cost Explorer can
// show what a cluster actually cost over time instead of a run rate.
package costhistory

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"kube-budget/core/costmodel"
)

// SchemaVersion guards stored records against model changes. Records written
// by a different version are ignored rather than misread.
const SchemaVersion = 1

// defaultMaxRecords bounds the store so a long-running desktop app cannot grow
// the file without limit.
const defaultMaxRecords = 20000

// Record is one capture. Only hourly rates are stored; every other window is
// derived through costmodel.Project so the conversion stays in one place.
type Record struct {
	SchemaVersion     int                `json:"schemaVersion"`
	ClusterID         string             `json:"clusterId"`
	CapturedAt        time.Time          `json:"capturedAt"`
	Currency          string             `json:"currency"`
	HourlyByBasis     map[string]float64 `json:"hourlyByBasis"`
	IdleHourly        float64            `json:"idleHourly"`
	HourlyByNamespace map[string]float64 `json:"hourlyByNamespace,omitempty"`
	HourlyByNodeGroup map[string]float64 `json:"hourlyByNodeGroup,omitempty"`
	NodeCount         int                `json:"nodeCount"`
	WorkloadCount     int                `json:"workloadCount"`
	Unpriced          int                `json:"unpriced"`
}

// Store appends records to a newline-delimited JSON file owned by the user.
type Store struct {
	path       string
	maxRecords int
	mutex      sync.Mutex
}

// New creates a store backed by the given file.
func New(path string) *Store {
	return &Store{path: path, maxRecords: defaultMaxRecords}
}

// DefaultPath returns the per-user location of the cost history file.
func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cost history: resolve config directory: %w", err)
	}
	return filepath.Join(configDir, "kube-budget", "cost-history.ndjson"), nil
}

// ClusterID builds a stable key for a report scope.
func ClusterID(scope costmodel.Scope) string {
	parts := []string{scope.Provider, scope.Region, scope.ClusterName}
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	if len(cleaned) == 0 {
		return "unknown"
	}
	return strings.Join(cleaned, "/")
}

// FromReport projects a report into the compact record shape.
func FromReport(report costmodel.CostReport, capturedAt time.Time) Record {
	record := Record{
		SchemaVersion:     SchemaVersion,
		ClusterID:         ClusterID(report.Scope),
		CapturedAt:        capturedAt.UTC(),
		Currency:          report.Currency,
		HourlyByBasis:     make(map[string]float64, len(report.Totals)),
		IdleHourly:        report.Idle.Hourly,
		HourlyByNamespace: hourlyDimension(report, costmodel.DimensionNamespace),
		HourlyByNodeGroup: hourlyDimension(report, costmodel.DimensionParent),
	}
	for basis, projection := range report.Totals {
		record.HourlyByBasis[string(basis)] = projection.Hourly
	}
	for _, item := range report.Items {
		switch item.Subject.Kind {
		case costmodel.SubjectNode:
			record.NodeCount++
		case costmodel.SubjectWorkload:
			record.WorkloadCount++
		}
		if !item.Priced() {
			record.Unpriced++
		}
	}
	return record
}

// Append writes one record. The file and its directory are private to the user.
func (store *Store) Append(record Record) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	if err := os.MkdirAll(filepath.Dir(store.path), 0o700); err != nil {
		return fmt.Errorf("cost history: create directory: %w", err)
	}
	file, err := os.OpenFile(store.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("cost history: open store: %w", err)
	}
	defer file.Close()

	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("cost history: encode record: %w", err)
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("cost history: write record: %w", err)
	}

	return store.trimLocked()
}

// Range returns the records of one cluster inside a time window, oldest first.
func (store *Store) Range(clusterID string, from, to time.Time) ([]Record, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	records, err := store.readLocked()
	if err != nil {
		return nil, err
	}

	matching := make([]Record, 0, len(records))
	for _, record := range records {
		if clusterID != "" && record.ClusterID != clusterID {
			continue
		}
		if record.CapturedAt.Before(from) || record.CapturedAt.After(to) {
			continue
		}
		matching = append(matching, record)
	}
	sort.Slice(matching, func(i, j int) bool { return matching[i].CapturedAt.Before(matching[j].CapturedAt) })
	return matching, nil
}

// Clusters lists the cluster IDs present in the store.
func (store *Store) Clusters() ([]string, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	records, err := store.readLocked()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	ids := make([]string, 0)
	for _, record := range records {
		if !seen[record.ClusterID] {
			seen[record.ClusterID] = true
			ids = append(ids, record.ClusterID)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// readLocked loads every readable record. Corrupt or foreign-version lines are
// skipped so one bad line cannot make the whole history unreadable.
func (store *Store) readLocked() ([]Record, error) {
	file, err := os.Open(store.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cost history: open store: %w", err)
	}
	defer file.Close()

	records := make([]Record, 0)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record Record
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		if record.SchemaVersion != SchemaVersion {
			continue
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("cost history: read store: %w", err)
	}
	return records, nil
}

func (store *Store) trimLocked() error {
	records, err := store.readLocked()
	if err != nil {
		return err
	}
	if len(records) <= store.maxRecords {
		return nil
	}

	sort.Slice(records, func(i, j int) bool { return records[i].CapturedAt.Before(records[j].CapturedAt) })
	kept := records[len(records)-store.maxRecords:]

	temporary := store.path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("cost history: rewrite store: %w", err)
	}
	writer := bufio.NewWriter(file)
	for _, record := range kept {
		encoded, err := json.Marshal(record)
		if err != nil {
			file.Close()
			return fmt.Errorf("cost history: encode record: %w", err)
		}
		if _, err := writer.Write(append(encoded, '\n')); err != nil {
			file.Close()
			return fmt.Errorf("cost history: rewrite store: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		file.Close()
		return fmt.Errorf("cost history: rewrite store: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("cost history: rewrite store: %w", err)
	}
	return os.Rename(temporary, store.path)
}

func hourlyDimension(report costmodel.CostReport, dimension costmodel.Dimension) map[string]float64 {
	buckets := report.ByDimension[dimension]
	if len(buckets) == 0 {
		return nil
	}
	hourly := make(map[string]float64, len(buckets))
	for key, projection := range buckets {
		hourly[key] = projection.Hourly
	}
	return hourly
}
