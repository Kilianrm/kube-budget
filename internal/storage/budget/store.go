// Package budget keeps each cluster's monthly budget in a small JSON file
// owned by the user, next to the cost history.
package budget

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Budget is the monthly limit set for one cluster.
type Budget struct {
	MonthlyUSD float64   `json:"monthlyUSD"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Store reads and writes budgets keyed by cluster ID.
type Store struct {
	path  string
	mutex sync.Mutex
}

// New creates a store backed by the given file.
func New(path string) *Store {
	return &Store{path: path}
}

// DefaultPath returns the per-user location of the budgets file.
func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("budget: resolve config directory: %w", err)
	}
	return filepath.Join(configDir, "kube-budget", "budgets.json"), nil
}

// Get returns a cluster's budget and whether one is set.
func (store *Store) Get(clusterID string) (Budget, bool, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	budgets, err := store.readLocked()
	if err != nil {
		return Budget{}, false, err
	}
	budget, ok := budgets[clusterID]
	return budget, ok, nil
}

// Set stores a cluster's monthly budget. A zero or negative amount removes it.
func (store *Store) Set(clusterID string, monthlyUSD float64, at time.Time) error {
	if clusterID == "" {
		return fmt.Errorf("budget: cluster ID is required")
	}

	store.mutex.Lock()
	defer store.mutex.Unlock()

	budgets, err := store.readLocked()
	if err != nil {
		return err
	}
	if monthlyUSD <= 0 {
		delete(budgets, clusterID)
	} else {
		budgets[clusterID] = Budget{MonthlyUSD: monthlyUSD, UpdatedAt: at.UTC()}
	}
	return store.writeLocked(budgets)
}

func (store *Store) readLocked() (map[string]Budget, error) {
	budgets := make(map[string]Budget)
	raw, err := os.ReadFile(store.path)
	if os.IsNotExist(err) {
		return budgets, nil
	}
	if err != nil {
		return nil, fmt.Errorf("budget: read store: %w", err)
	}
	if err := json.Unmarshal(raw, &budgets); err != nil {
		return nil, fmt.Errorf("budget: decode store: %w", err)
	}
	return budgets, nil
}

// writeLocked replaces the file through a rename so a crash never leaves it
// half written.
func (store *Store) writeLocked(budgets map[string]Budget) error {
	if err := os.MkdirAll(filepath.Dir(store.path), 0o700); err != nil {
		return fmt.Errorf("budget: create directory: %w", err)
	}
	encoded, err := json.MarshalIndent(budgets, "", "  ")
	if err != nil {
		return fmt.Errorf("budget: encode store: %w", err)
	}
	temporary := store.path + ".tmp"
	if err := os.WriteFile(temporary, encoded, 0o600); err != nil {
		return fmt.Errorf("budget: write store: %w", err)
	}
	return os.Rename(temporary, store.path)
}
