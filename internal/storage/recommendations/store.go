// Package recommendations remembers what the user did with optimization
// recommendations: which ones were dismissed, and which were applied together
// with the billed rate at that moment, so the realized saving can be checked.
// Every action is also kept in a history log.
package recommendations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Applied is one recommendation the user marked as applied.
type Applied struct {
	ID    string    `json:"id"`
	Title string    `json:"title"`
	At    time.Time `json:"at"`
	// BaselineHourly is the billed rate when it was marked applied.
	BaselineHourly float64 `json:"baselineHourly"`
	// ExpectedHourly is the saving the plan claimed at that moment.
	ExpectedHourly float64 `json:"expectedHourly"`
	// ConfirmedAt is when a report first stopped detecting it. Until then the
	// change is pending, and marking it again does nothing.
	ConfirmedAt *time.Time `json:"confirmedAt,omitempty"`
}

// Pending reports whether the change has not been confirmed by a report yet.
func (applied Applied) Pending() bool {
	return applied.ConfirmedAt == nil
}

// Action is what happened to a recommendation.
type Action string

const (
	ActionApplied   Action = "applied"
	ActionConfirmed Action = "confirmed"
	ActionReopened  Action = "reopened"
	ActionDismissed Action = "dismissed"
	ActionRestored  Action = "restored"
)

// Event is one entry of the history log.
type Event struct {
	At     time.Time `json:"at"`
	ID     string    `json:"id"`
	Title  string    `json:"title"`
	Action Action    `json:"action"`
	// SavingsHourly is the saving the plan claimed at that moment.
	SavingsHourly float64 `json:"savingsHourly"`
}

// State is everything stored for one cluster.
type State struct {
	Dismissed map[string]time.Time `json:"dismissed,omitempty"`
	Applied   []Applied            `json:"applied,omitempty"`
	History   []Event              `json:"history,omitempty"`
}

// PendingApplied returns the applied entry still waiting for confirmation.
func (state State) PendingApplied(id string) (Applied, bool) {
	for _, applied := range state.Applied {
		if applied.ID == id && applied.Pending() {
			return applied, true
		}
	}
	return Applied{}, false
}

// maxApplied bounds the applied journal and maxHistory the history log, per cluster.
const (
	maxApplied = 50
	maxHistory = 200
)

// Store keeps each cluster's state in one JSON file owned by the user.
type Store struct {
	path  string
	mutex sync.Mutex
}

// New creates a store backed by the given file.
func New(path string) *Store {
	return &Store{path: path}
}

// DefaultPath returns the per-user location of the recommendations file.
func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("recommendations: resolve config directory: %w", err)
	}
	return filepath.Join(configDir, "kube-budget", "recommendations.json"), nil
}

// Get returns a cluster's state; an unknown cluster has an empty one.
func (store *Store) Get(clusterID string) (State, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	states, err := store.readLocked()
	if err != nil {
		return State{}, err
	}
	return states[clusterID], nil
}

// Dismiss hides a recommendation from the plan until it is restored.
func (store *Store) Dismiss(clusterID string, event Event) error {
	return store.update(clusterID, func(state *State) {
		if _, dismissed := state.Dismissed[event.ID]; dismissed {
			return
		}
		if state.Dismissed == nil {
			state.Dismissed = make(map[string]time.Time)
		}
		event.Action = ActionDismissed
		state.Dismissed[event.ID] = event.At.UTC()
		state.log(event)
	})
}

// Restore brings a dismissed recommendation back into the plan.
func (store *Store) Restore(clusterID string, event Event) error {
	return store.update(clusterID, func(state *State) {
		if _, dismissed := state.Dismissed[event.ID]; !dismissed {
			return
		}
		delete(state.Dismissed, event.ID)
		event.Action = ActionRestored
		state.log(event)
	})
}

// MarkApplied records that a recommendation was applied. A recommendation
// already pending is left as it is, so repeated clicks record it once. The
// newest entry comes first and the journal keeps the latest maxApplied.
func (store *Store) MarkApplied(clusterID string, applied Applied) error {
	applied.At = applied.At.UTC()
	return store.update(clusterID, func(state *State) {
		if _, pending := state.PendingApplied(applied.ID); pending {
			return
		}
		state.Applied = append([]Applied{applied}, state.Applied...)
		if len(state.Applied) > maxApplied {
			state.Applied = state.Applied[:maxApplied]
		}
		state.log(Event{At: applied.At, ID: applied.ID, Title: applied.Title, Action: ActionApplied, SavingsHourly: applied.ExpectedHourly})
	})
}

// Reopen withdraws a pending applied mark, for a change that was not made
// after all. The recommendation is open again.
func (store *Store) Reopen(clusterID string, event Event) error {
	return store.update(clusterID, func(state *State) {
		kept := state.Applied[:0]
		removed := false
		for _, applied := range state.Applied {
			if applied.ID == event.ID && applied.Pending() {
				removed = true
				continue
			}
			kept = append(kept, applied)
		}
		if !removed {
			return
		}
		state.Applied = kept
		event.Action = ActionReopened
		state.log(event)
	})
}

// Confirm marks every pending change whose recommendation a report no longer
// detects as confirmed. It reports whether anything changed.
func (store *Store) Confirm(clusterID string, detected map[string]bool, at time.Time) (bool, error) {
	confirmed := false
	err := store.update(clusterID, func(state *State) {
		for index, applied := range state.Applied {
			if !applied.Pending() || detected[applied.ID] {
				continue
			}
			when := at.UTC()
			state.Applied[index].ConfirmedAt = &when
			state.log(Event{At: when, ID: applied.ID, Title: applied.Title, Action: ActionConfirmed, SavingsHourly: applied.ExpectedHourly})
			confirmed = true
		}
	})
	return confirmed, err
}

// log adds an event, newest first, keeping the latest maxHistory.
func (state *State) log(event Event) {
	event.At = event.At.UTC()
	state.History = append([]Event{event}, state.History...)
	if len(state.History) > maxHistory {
		state.History = state.History[:maxHistory]
	}
}

func (store *Store) update(clusterID string, change func(*State)) error {
	if clusterID == "" {
		return fmt.Errorf("recommendations: cluster ID is required")
	}

	store.mutex.Lock()
	defer store.mutex.Unlock()

	states, err := store.readLocked()
	if err != nil {
		return err
	}
	state := states[clusterID]
	before, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("recommendations: encode state: %w", err)
	}
	change(&state)
	after, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("recommendations: encode state: %w", err)
	}
	// Reports confirm changes on every refresh; most refreshes change nothing.
	if bytes.Equal(before, after) {
		return nil
	}
	states[clusterID] = state
	return store.writeLocked(states)
}

func (store *Store) readLocked() (map[string]State, error) {
	states := make(map[string]State)
	raw, err := os.ReadFile(store.path)
	if os.IsNotExist(err) {
		return states, nil
	}
	if err != nil {
		return nil, fmt.Errorf("recommendations: read store: %w", err)
	}
	if err := json.Unmarshal(raw, &states); err != nil {
		return nil, fmt.Errorf("recommendations: decode store: %w", err)
	}
	return states, nil
}

func (store *Store) writeLocked(states map[string]State) error {
	if err := os.MkdirAll(filepath.Dir(store.path), 0o700); err != nil {
		return fmt.Errorf("recommendations: create directory: %w", err)
	}
	encoded, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return fmt.Errorf("recommendations: encode store: %w", err)
	}
	temporary := store.path + ".tmp"
	if err := os.WriteFile(temporary, encoded, 0o600); err != nil {
		return fmt.Errorf("recommendations: write store: %w", err)
	}
	return os.Rename(temporary, store.path)
}
