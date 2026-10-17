package recommendations

import (
	"bytes"
	"encoding/json"
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

	changed, err := store.Confirm("demo", map[string]Seen{"consolidate-nodes": {Title: "Run on one fewer node"}}, at.Add(time.Minute), at.Add(time.Minute))
	if err != nil || changed {
		t.Fatalf("Confirm() = %v, %v; want nothing confirmed while still detected", changed, err)
	}
	changed, _ = store.Confirm("demo", map[string]Seen{}, at.Add(time.Hour), at.Add(time.Hour))
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

func TestARecommendationThatDisappearsIsLoggedNotApplied(t *testing.T) {
	store := newStore(t)
	seen := map[string]Seen{
		"orphan-volumes": {Title: "Delete 1 volume nothing uses", SavingsHourly: 0.005, BilledHourly: 0.5},
		"spot-capacity":  {Title: "Run stateless workloads on spot", SavingsHourly: 0.06, BilledHourly: 0.5},
	}
	_, _ = store.Confirm("demo", seen, at, at)
	_ = store.Dismiss("demo", Event{ID: "spot-capacity", At: at.Add(time.Minute)})

	// one report without them: not logged yet, it may be a flicker
	if changed, _ := store.Confirm("demo", map[string]Seen{}, at.Add(time.Hour), at.Add(time.Hour)); changed {
		t.Errorf("Confirm() changed after one report without it, want a second report first")
	}
	// a second one: logged as gone, never recorded as applied
	changed, _ := store.Confirm("demo", map[string]Seen{}, at.Add(2*time.Hour), at.Add(2*time.Hour))
	state, _ := store.Get("demo")
	if !changed || len(state.Applied) != 0 || state.History[0].Action != ActionGone || state.History[0].ID != "orphan-volumes" {
		t.Fatalf("applied = %+v, history = %+v; want only a gone entry for the volume", state.Applied, state.History)
	}
	if absence := state.Absent["orphan-volumes"]; !absence.Logged || !absence.LastSeen.Equal(at) || len(state.Absent) != 1 {
		t.Errorf("absent = %+v, want the volume, last seen at the first report; the dismissed step is not tracked", state.Absent)
	}
	// and it is logged once
	if changed, _ := store.Confirm("demo", map[string]Seen{}, at.Add(3*time.Hour), at.Add(3*time.Hour)); changed {
		t.Error("Confirm() logged the same absence twice")
	}
}

func TestARecommendationThatFlickersIsNotLogged(t *testing.T) {
	store := newStore(t)
	seen := map[string]Seen{"rightsize-requests": {Title: "Rightsize 1 workload"}}
	_, _ = store.Confirm("demo", seen, at, at)
	_, _ = store.Confirm("demo", map[string]Seen{}, at.Add(time.Hour), at.Add(time.Hour))
	_, _ = store.Confirm("demo", seen, at.Add(2*time.Hour), at.Add(2*time.Hour))

	if state, _ := store.Get("demo"); len(state.History) != 0 || len(state.Absent) != 0 {
		t.Errorf("history = %+v, absent = %+v; want nothing for a single missed report", state.History, state.Absent)
	}
}

func TestAGoneRecommendationThatComesBackIsLoggedAgain(t *testing.T) {
	store := newStore(t)
	seen := map[string]Seen{"orphan-volumes": {Title: "Delete 1 volume nothing uses"}}
	_, _ = store.Confirm("demo", seen, at, at)
	_, _ = store.Confirm("demo", map[string]Seen{}, at.Add(time.Hour), at.Add(time.Hour))
	_, _ = store.Confirm("demo", map[string]Seen{}, at.Add(2*time.Hour), at.Add(2*time.Hour))
	_, _ = store.Confirm("demo", seen, at.Add(3*time.Hour), at.Add(3*time.Hour))

	state, _ := store.Get("demo")
	if state.History[0].Action != ActionReturned || len(state.Absent) != 0 {
		t.Errorf("history = %+v, absent = %+v; want it logged as detected again", state.History, state.Absent)
	}
	if err := store.Claim("demo", "orphan-volumes", at.Add(4*time.Hour)); err == nil {
		t.Error("Claim() of a recommendation detected again, want an error")
	}
}

func TestTheUserCanClaimAGoneRecommendation(t *testing.T) {
	store := newStore(t)
	seen := map[string]Seen{"orphan-volumes": {Title: "Delete 1 volume nothing uses", SavingsHourly: 0.005, BilledHourly: 0.5, Details: []byte(`{"id":"orphan-volumes"}`)}}
	_, _ = store.Confirm("demo", seen, at, at)
	if err := store.Claim("demo", "orphan-volumes", at.Add(time.Minute)); err == nil {
		t.Error("Claim() before it was logged as gone, want an error")
	}
	_, _ = store.Confirm("demo", map[string]Seen{}, at.Add(time.Hour), at.Add(time.Hour))
	_, _ = store.Confirm("demo", map[string]Seen{}, at.Add(2*time.Hour), at.Add(2*time.Hour))

	if err := store.Claim("demo", "orphan-volumes", at.Add(3*time.Hour)); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	state, _ := store.Get("demo")
	claimed := state.Applied[0]
	if !claimed.Unmarked || claimed.Pending() || !claimed.At.Equal(at) || claimed.ExpectedHourly != 0.005 || claimed.BaselineHourly != 0.5 || len(claimed.Details) == 0 {
		t.Errorf("claimed = %+v, want a confirmed unmarked entry dated when it was last seen", claimed)
	}
	if state.History[0].Action != ActionClaimed || len(state.Absent) != 0 {
		t.Errorf("history = %+v, want the claim logged and the absence closed", state.History[0])
	}
}

func TestOnlyAChangeBeingAppliedTracksItsItems(t *testing.T) {
	store := newStore(t)
	two := map[string]Seen{"orphan-volumes": {Title: "Delete volumes", Items: []SeenItem{{Key: "volume/prod/a", Name: "a"}, {Key: "volume/prod/b", Name: "b"}}}}
	one := map[string]Seen{"orphan-volumes": {Title: "Delete volumes", Items: []SeenItem{{Key: "volume/prod/b", Name: "b"}}}}
	_, _ = store.Confirm("demo", two, at, at)
	_, _ = store.Confirm("demo", one, at.Add(time.Hour), at.Add(time.Hour))

	if state, _ := store.Get("demo"); len(state.Progress) != 0 || len(state.History) != 0 {
		t.Errorf("progress = %+v, history = %+v; want nothing tracked without the user applying it", state.Progress, state.History)
	}
}

func TestAMarkedChangeIsConfirmedNotRecordedTwice(t *testing.T) {
	store := newStore(t)
	_, _ = store.Confirm("demo", map[string]Seen{"orphan-volumes": {Title: "Delete volumes"}}, at, at)
	_ = store.MarkApplied("demo", Applied{ID: "orphan-volumes", Title: "Delete volumes", At: at.Add(time.Minute)})

	_, _ = store.Confirm("demo", map[string]Seen{}, at.Add(time.Hour), at.Add(time.Hour))
	state, _ := store.Get("demo")
	if len(state.Applied) != 1 || state.Applied[0].Unmarked || state.Applied[0].Pending() {
		t.Errorf("applied = %+v, want the one marked entry, confirmed", state.Applied)
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
	if _, err := store.Confirm("demo", map[string]Seen{}, at, at); err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	written, err := os.Stat(store.path)
	if err != nil {
		t.Fatalf("the first report was not recorded: %v", err)
	}
	// the same report planned again, as acting on a recommendation does
	if _, err := store.Confirm("demo", map[string]Seen{}, at, at.Add(time.Minute)); err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if again, _ := os.Stat(store.path); !again.ModTime().Equal(written.ModTime()) {
		t.Errorf("store rewritten for the same report, want no write")
	}
}

func TestReplanningTheSameReportIsNotAnotherReport(t *testing.T) {
	store := newStore(t)
	_, _ = store.Confirm("demo", map[string]Seen{"orphan-volumes": {Title: "Delete 1 volume nothing uses"}}, at, at)
	// one report without it, planned three times over by clicks
	for minute := 0; minute < 3; minute++ {
		_, _ = store.Confirm("demo", map[string]Seen{}, at.Add(time.Hour), at.Add(time.Hour+time.Duration(minute)*time.Minute))
	}
	state, _ := store.Get("demo")
	if len(state.History) != 0 || state.Absent["orphan-volumes"].Reports != 1 {
		t.Errorf("history = %+v, absent = %+v; want one report counted and nothing logged", state.History, state.Absent)
	}
	if len(state.Previous) != 1 || !state.PreviousAt.Equal(at) {
		t.Errorf("previous = %+v at %v, want the first report kept as the one before", state.Previous, state.PreviousAt)
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

func TestItemsDoneOneAtATimeAreRecordedAndCarriedToTheConfirmedChange(t *testing.T) {
	store := newStore(t)
	item := func(name string) SeenItem {
		return SeenItem{Key: "workload/shop/" + name, Name: name, Namespace: "shop", SavingsHourly: 0.01}
	}
	rightsize := func(names ...string) map[string]Seen {
		seen := Seen{Title: "Rightsize workloads", Details: []byte(`{"id":"rightsize-requests"}`)}
		for _, name := range names {
			seen.Items = append(seen.Items, item(name))
		}
		return map[string]Seen{"rightsize-requests": seen}
	}

	_, _ = store.Confirm("demo", rightsize("api", "db", "agent"), at, at)
	_ = store.MarkApplied("demo", Applied{ID: "rightsize-requests", Title: "Rightsize workloads", At: at})

	// agent fixed first: recorded while the recommendation stays pending
	changed, _ := store.Confirm("demo", rightsize("api", "db"), at.Add(time.Hour), at.Add(time.Hour))
	state, _ := store.Get("demo")
	if !changed || len(state.Progress["rightsize-requests"]) != 1 || state.Progress["rightsize-requests"][0].Name != "agent" {
		t.Fatalf("progress = %+v, want agent done", state.Progress)
	}
	if state.History[0].Action != ActionItemDone || !state.Applied[0].Pending() {
		t.Errorf("history = %+v, applied = %+v; want the item logged and the change still pending", state.History[0], state.Applied[0])
	}

	// db comes back after being gone: no longer counted as done
	_, _ = store.Confirm("demo", rightsize("api"), at.Add(2*time.Hour), at.Add(2*time.Hour))
	_, _ = store.Confirm("demo", rightsize("api", "db"), at.Add(3*time.Hour), at.Add(3*time.Hour))
	if state, _ := store.Get("demo"); len(state.Progress["rightsize-requests"]) != 1 {
		t.Errorf("progress = %+v, want only agent after db came back", state.Progress)
	}

	// the rest fixed: confirmed with every item and the last details
	_, _ = store.Confirm("demo", map[string]Seen{}, at.Add(4*time.Hour), at.Add(4*time.Hour))
	state, _ = store.Get("demo")
	finished := state.Applied[0]
	if finished.Pending() || len(finished.Done) != 3 || finished.Done[0].Name != "agent" || !finished.Done[0].At.Equal(at.Add(time.Hour)) {
		t.Errorf("done = %+v, want agent first with its own date, then api and db", finished.Done)
	}
	var details bytes.Buffer
	_ = json.Compact(&details, finished.Details)
	if details.String() != `{"id":"rightsize-requests"}` || len(state.Progress) != 0 {
		t.Errorf("details = %s, progress = %+v; want the details kept and the progress cleared", finished.Details, state.Progress)
	}
}
