// Package recommendations remembers what the user did with optimization
// recommendations: which ones were dismissed, and which were applied together
// with the billed rate at that moment, so the realized saving can be checked.
// It also remembers what the last report detected. A recommendation that
// stops being detected without being applied is not assumed applied: it is
// logged as no longer detected, and the user may claim it. Every action is
// also kept in a history log.
package recommendations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	// Unmarked is set when the user claimed a recommendation that stopped
	// being detected without being applied: the change was made outside
	// KubeBudget. At is when it was last detected, and the rates are the ones
	// seen then.
	Unmarked bool `json:"unmarked,omitempty"`
	// Details is the recommendation as it was last detected, kept opaque so
	// the confirmed change can still be shown in full.
	Details json.RawMessage `json:"details,omitempty"`
	// Done lists the items that stopped being detected, each when it did.
	Done []DoneItem `json:"done,omitempty"`
}

// Absence is a recommendation that stopped being detected. It is logged as
// gone once goneAfter reports in a row miss it, which keeps a recommendation
// flickering in and out of the plan out of the log, and can then be claimed.
type Absence struct {
	Seen
	// LastSeen is when a report last detected it.
	LastSeen time.Time `json:"lastSeen"`
	Reports  int       `json:"reports"`
	Logged   bool      `json:"logged,omitempty"`
}

// goneAfter is how many reports in a row must miss a recommendation before
// it is logged as no longer detected; absenceKept bounds how long it can be
// claimed.
const (
	goneAfter   = 2
	absenceKept = 30 * 24 * time.Hour
)

// Seen is a recommendation as a report detected it.
type Seen struct {
	Title         string  `json:"title"`
	SavingsHourly float64 `json:"savingsHourly"`
	// BilledHourly is the billed rate of that report.
	BilledHourly float64 `json:"billedHourly"`
	// Items are tracked one by one only when each is a change of its own.
	Items   []SeenItem      `json:"items,omitempty"`
	Details json.RawMessage `json:"details,omitempty"`
}

// SeenItem is one resource of a recommendation.
type SeenItem struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	// SavingsHourly is what the plan claimed for this item.
	SavingsHourly float64 `json:"savingsHourly"`
	// Details is the item as last detected, kept opaque like the
	// recommendation's, so a done item still shows its change and command.
	Details json.RawMessage `json:"details,omitempty"`
}

// DoneItem is an item a report stopped detecting.
type DoneItem struct {
	SeenItem
	At time.Time `json:"at"`
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
	// ActionResolved was written by versions that recorded a recommendation
	// as applied as soon as it disappeared; it is kept for their logs.
	ActionResolved  Action = "resolved"
	ActionItemDone  Action = "item-done"
	ActionGone      Action = "gone"
	ActionReturned  Action = "returned"
	ActionClaimed   Action = "claimed"
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
	// Detected is what the last report detected, by recommendation ID, and
	// DetectedAt when that report was generated. Previous is what the report
	// before it detected, so the plan can say what changed since.
	Detected   map[string]Seen `json:"detected,omitempty"`
	DetectedAt time.Time       `json:"detectedAt,omitempty"`
	Previous   map[string]Seen `json:"previous,omitempty"`
	PreviousAt time.Time       `json:"previousAt,omitempty"`
	// Progress holds the items already done of recommendations being applied.
	Progress map[string][]DoneItem `json:"progress,omitempty"`
	// Absent holds recommendations that stopped being detected without being
	// applied or dismissed, by ID.
	Absent  map[string]Absence `json:"absent,omitempty"`
	History []Event            `json:"history,omitempty"`
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

// Confirm records what a report detects. A change being applied that the
// report no longer detects is confirmed, with its items. A recommendation that
// disappears without being applied or dismissed is only logged: as gone once
// goneAfter reports in a row miss it, and as detected again if it returns.
// A report is known by when it was generated: acting on a recommendation
// replans the same report, which must not count as another one. It reports
// whether the applied journal, the progress or the log changed.
func (store *Store) Confirm(clusterID string, detected map[string]Seen, reportAt, at time.Time) (bool, error) {
	changed := false
	err := store.update(clusterID, func(state *State) {
		when := at.UTC()
		reportAt = reportAt.UTC()
		newReport := !reportAt.Equal(state.DetectedAt)
		marked := make(map[string]bool)
		for _, applied := range state.Applied {
			if applied.Pending() {
				marked[applied.ID] = true
			}
		}
		// Only a change being applied tracks its items: without the user's
		// intent, an item dropping out says nothing about who changed it.
		for id := range state.Progress {
			if !marked[id] {
				state.setProgress(id, nil)
			}
		}
		for _, id := range sortedKeys(detected) {
			if marked[id] && state.trackItems(id, detected[id], when) {
				changed = true
			}
		}

		for index, applied := range state.Applied {
			if !applied.Pending() {
				continue
			}
			if _, still := detected[applied.ID]; still {
				continue
			}
			finished := &state.Applied[index]
			finished.ConfirmedAt = &when
			finished.Done = state.finishItems(applied.ID, when)
			if last, ok := state.Detected[applied.ID]; ok && len(last.Details) > 0 {
				finished.Details = last.Details
			}
			state.log(Event{At: when, ID: applied.ID, Title: applied.Title, Action: ActionConfirmed, SavingsHourly: applied.ExpectedHourly})
			changed = true
		}
		if newReport && state.trackAbsences(detected, marked, when) {
			changed = true
		}

		if len(state.Applied) > maxApplied {
			state.Applied = state.Applied[:maxApplied]
		}
		if newReport && !state.DetectedAt.IsZero() {
			state.Previous, state.PreviousAt = state.Detected, state.DetectedAt
		}
		state.Detected, state.DetectedAt = nil, reportAt
		if len(detected) > 0 {
			state.Detected = detected
		}
	})
	return changed, err
}

// trackAbsences follows the recommendations that stopped being detected
// without being applied or dismissed.
func (state *State) trackAbsences(detected map[string]Seen, marked map[string]bool, at time.Time) bool {
	changed := false
	for _, id := range sortedKeys(state.Absent) {
		absence := state.Absent[id]
		_, back := detected[id]
		_, dismissed := state.Dismissed[id]
		switch {
		case back:
			if absence.Logged {
				state.log(Event{At: at, ID: id, Title: absence.Title, Action: ActionReturned, SavingsHourly: detected[id].SavingsHourly})
				changed = true
			}
			delete(state.Absent, id)
		case dismissed || marked[id] || at.Sub(absence.LastSeen) > absenceKept:
			delete(state.Absent, id)
		default:
			absence.Reports++
			if absence.Reports >= goneAfter && !absence.Logged {
				absence.Logged = true
				state.log(Event{At: at, ID: id, Title: absence.Title, Action: ActionGone, SavingsHourly: absence.SavingsHourly})
				changed = true
			}
			state.Absent[id] = absence
		}
	}
	for _, id := range sortedKeys(state.Detected) {
		_, still := detected[id]
		_, dismissed := state.Dismissed[id]
		if _, tracked := state.Absent[id]; still || dismissed || marked[id] || tracked {
			continue
		}
		if state.Absent == nil {
			state.Absent = make(map[string]Absence)
		}
		lastSeen := state.DetectedAt
		if lastSeen.IsZero() {
			lastSeen = at
		}
		state.Absent[id] = Absence{Seen: state.Detected[id], LastSeen: lastSeen, Reports: 1}
	}
	if len(state.Absent) == 0 {
		state.Absent = nil
	}
	return changed
}

// Claim records a recommendation logged as no longer detected as applied
// outside KubeBudget, when the user says the change was theirs. It is
// confirmed at once, with the rates last seen while it was detected.
func (store *Store) Claim(clusterID, id string, at time.Time) error {
	claimed := false
	err := store.update(clusterID, func(state *State) {
		absence, ok := state.Absent[id]
		if !ok || !absence.Logged {
			return
		}
		when := at.UTC()
		state.Applied = append([]Applied{{
			ID:             id,
			Title:          absence.Title,
			At:             absence.LastSeen,
			BaselineHourly: absence.BilledHourly,
			ExpectedHourly: absence.SavingsHourly,
			ConfirmedAt:    &when,
			Unmarked:       true,
			Details:        absence.Details,
		}}, state.Applied...)
		if len(state.Applied) > maxApplied {
			state.Applied = state.Applied[:maxApplied]
		}
		delete(state.Absent, id)
		if len(state.Absent) == 0 {
			state.Absent = nil
		}
		state.log(Event{At: when, ID: id, Title: absence.Title, Action: ActionClaimed, SavingsHourly: absence.SavingsHourly})
		claimed = true
	})
	if err == nil && !claimed {
		return fmt.Errorf("recommendations: %q is not logged as no longer detected", id)
	}
	return err
}

// trackItems records the items of a still detected recommendation that the
// previous report listed and this one does not. An item that comes back is
// no longer counted as done.
func (state *State) trackItems(id string, now Seen, at time.Time) bool {
	previous, known := state.Detected[id]
	current := make(map[string]bool, len(now.Items))
	for _, item := range now.Items {
		current[item.Key] = true
	}

	changed := false
	kept := state.Progress[id][:0:0]
	for _, done := range state.Progress[id] {
		if current[done.Key] {
			changed = true
			continue
		}
		kept = append(kept, done)
	}
	if known {
		for _, item := range previous.Items {
			if current[item.Key] || containsItem(kept, item.Key) {
				continue
			}
			kept = append(kept, DoneItem{SeenItem: item, At: at})
			state.log(Event{At: at, ID: id, Title: previous.Title + " · " + itemName(item), Action: ActionItemDone, SavingsHourly: item.SavingsHourly})
			changed = true
		}
	}
	state.setProgress(id, kept)
	return changed
}

// finishItems closes a recommendation that is no longer detected: the items
// done before it plus the ones the last report still listed.
func (state *State) finishItems(id string, at time.Time) []DoneItem {
	done := state.Progress[id]
	for _, item := range state.Detected[id].Items {
		if !containsItem(done, item.Key) {
			done = append(done, DoneItem{SeenItem: item, At: at})
		}
	}
	state.setProgress(id, nil)
	return done
}

func (state *State) setProgress(id string, done []DoneItem) {
	if len(done) == 0 {
		delete(state.Progress, id)
		if len(state.Progress) == 0 {
			state.Progress = nil
		}
		return
	}
	if state.Progress == nil {
		state.Progress = make(map[string][]DoneItem)
	}
	state.Progress[id] = done
}

func containsItem(items []DoneItem, key string) bool {
	for _, item := range items {
		if item.Key == key {
			return true
		}
	}
	return false
}

func itemName(item SeenItem) string {
	if item.Namespace == "" {
		return item.Name
	}
	return item.Namespace + "/" + item.Name
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
