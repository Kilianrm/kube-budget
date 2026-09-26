package recommendations

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var at = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

func newStore(t *testing.T) *Store {
	return New(filepath.Join(t.TempDir(), "recommendations.json"))
}

func TestDismissRestoreAndApply(t *testing.T) {
	store := newStore(t)

	if err := store.Dismiss("demo", Event{ID: "spot-capacity", Title: "Use spot", At: at}); err != nil {
		t.Fatalf("Dismiss() error = %v", err)
	}
	if err := store.MarkApplied("demo", Applied{ID: "orphan-volumes", Title: "Delete 2 volumes", At: at, BaselineHourly: 1.5, ExpectedHourly: 0.1}); err != nil {
		t.Fatalf("MarkApplied() error = %v", err)
	}
	state, err := store.Get("demo")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if _, ok := state.Dismissed["spot-capacity"]; !ok || len(state.Applied) != 1 || state.Applied[0].BaselineHourly != 1.5 {
		t.Fatalf("state = %+v, want one dismissal and one applied entry", state)
	}

	if err := store.Restore("demo", Event{ID: "spot-capacity", At: at}); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	state, _ = store.Get("demo")
	if len(state.Dismissed) != 0 {
		t.Errorf("Dismissed = %+v after Restore(), want empty", state.Dismissed)
	}
	actions := []Action{}
	for _, event := range state.History {
		actions = append(actions, event.Action)
	}
	if len(actions) != 3 || actions[0] != ActionRestored || actions[1] != ActionApplied || actions[2] != ActionDismissed {
		t.Errorf("history = %v, want restored, applied, dismissed (newest first)", actions)
	}
	if state, _ := store.Get("other"); len(state.Applied) != 0 {
		t.Errorf("other cluster state = %+v, want empty", state)
	}
}

func TestMarkingAppliedTwiceRecordsItOnce(t *testing.T) {
	store := newStore(t)
	for range 3 {
		if err := store.MarkApplied("demo", Applied{ID: "consolidate-nodes", At: at}); err != nil {
			t.Fatalf("MarkApplied() error = %v", err)
		}
	}
	state, _ := store.Get("demo")
	if len(state.Applied) != 1 || len(state.History) != 1 {
		t.Errorf("applied = %d, history = %d; want one of each", len(state.Applied), len(state.History))
	}
}

func TestAChangeIsConfirmedOnceNoLongerDetected(t *testing.T) {
	store := newStore(t)
	_ = store.MarkApplied("demo", Applied{ID: "consolidate-nodes", At: at})

	changed, err := store.Confirm("demo", map[string]bool{"consolidate-nodes": true}, at.Add(time.Minute))
	if err != nil || changed {
		t.Fatalf("Confirm() = %v, %v; want nothing confirmed while still detected", changed, err)
	}
	changed, _ = store.Confirm("demo", map[string]bool{}, at.Add(time.Hour))
	state, _ := store.Get("demo")
	if !changed || state.Applied[0].Pending() || state.History[0].Action != ActionConfirmed {
		t.Fatalf("state = %+v, want the change confirmed and logged", state)
	}

	// once confirmed, the same recommendation can come back and be applied again
	_ = store.MarkApplied("demo", Applied{ID: "consolidate-nodes", At: at.Add(48 * time.Hour)})
	if state, _ := store.Get("demo"); len(state.Applied) != 2 || !state.Applied[0].Pending() {
		t.Errorf("applied = %+v, want a new pending entry next to the confirmed one", state.Applied)
	}
}

func TestReopenWithdrawsAPendingMark(t *testing.T) {
	store := newStore(t)
	_ = store.MarkApplied("demo", Applied{ID: "spot-capacity", At: at})

	if err := store.Reopen("demo", Event{ID: "spot-capacity", At: at.Add(time.Minute)}); err != nil {
		t.Fatalf("Reopen() error = %v", err)
	}
	state, _ := store.Get("demo")
	if _, pending := state.PendingApplied("spot-capacity"); pending || state.History[0].Action != ActionReopened {
		t.Errorf("state = %+v, want the mark withdrawn and logged", state)
	}
}

func TestAnUnchangedStateIsNotWritten(t *testing.T) {
	store := newStore(t)
	if _, err := store.Confirm("demo", map[string]bool{}, at); err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if _, err := os.Stat(store.path); !os.IsNotExist(err) {
		t.Errorf("store file exists after a no-op, want no write")
	}
}

func TestJournalsAreBounded(t *testing.T) {
	store := newStore(t)
	for index := 0; index < maxApplied+5; index++ {
		id := string(rune('a'+index%26)) + string(rune('a'+index/26))
		if err := store.MarkApplied("demo", Applied{ID: id, At: time.Unix(int64(index), 0)}); err != nil {
			t.Fatalf("MarkApplied() error = %v", err)
		}
	}
	state, _ := store.Get("demo")
	if len(state.Applied) != maxApplied || state.Applied[0].At.Unix() != maxApplied+4 {
		t.Errorf("journal = %d entries starting at %v, want %d newest first", len(state.Applied), state.Applied[0].At, maxApplied)
	}
}
