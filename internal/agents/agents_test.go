package agents

import (
	"path/filepath"
	"testing"
	"time"
)

// The whole reason this package exists: two agents in one worktree used to
// overwrite each other's state, so a den could only ever show one of them.
func TestTwoAgentsShareADen(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for _, session := range []string{"aaaa-1111", "bbbb-2222"} {
		if err := Touch(dir, Record{Session: session, Den: "demo#feat", Root: dir}, now); err != nil {
			t.Fatal(err)
		}
	}
	if living := InDen(dir, "demo#feat", now); len(living) != 2 {
		t.Fatalf("den holds %d agents, want 2", len(living))
	}
	if other := InDen(dir, "demo#elsewhere", now); len(other) != 0 {
		t.Errorf("agents leaked into another den: %d", len(other))
	}
}

// An agent that exits never says so, so silence has to be what marks it stale,
// and a long enough silence has to remove it entirely.
func TestSilenceGoesStaleThenForgotten(t *testing.T) {
	dir := t.TempDir()
	start := time.Now()
	if err := Touch(dir, Record{Session: "aaaa-1111", Den: "demo#feat", Root: dir}, start); err != nil {
		t.Fatal(err)
	}
	if All(dir, start)[0].Stale(start) {
		t.Error("a just-seen agent is stale")
	}
	later := start.Add(StaleAfter + time.Second)
	if !All(dir, later)[0].Stale(later) {
		t.Error("a silent agent never went stale")
	}
	if living := All(dir, start.Add(Forgotten+time.Minute)); len(living) != 0 {
		t.Errorf("a long-silent agent was still listed: %d", len(living))
	}
}

// An agent can move between worktrees. Its identity travels with it, it leaves
// no ghost behind, and the arrival time resets so travel is visible.
func TestAgentJumpsWorktrees(t *testing.T) {
	dir := t.TempDir()
	start := time.Now()
	if err := Touch(dir, Record{Session: "aaaa-1111", Den: "demo#first", Root: dir}, start); err != nil {
		t.Fatal(err)
	}
	// Well inside the heartbeat window: a move must still be written at once,
	// or the arrival is lost and the agent appears in the wrong den.
	moved := start.Add(2 * time.Second)
	if err := Touch(dir, Record{Session: "aaaa-1111", Den: "demo#second", Root: dir}, moved); err != nil {
		t.Fatal(err)
	}
	if left := InDen(dir, "demo#first", moved); len(left) != 0 {
		t.Errorf("a ghost stayed behind in the old den: %d", len(left))
	}
	arrived := InDen(dir, "demo#second", moved)
	if len(arrived) != 1 {
		t.Fatalf("agent did not arrive in the new den: %d", len(arrived))
	}
	if !arrived[0].JustArrived(moved) {
		t.Error("arrival time did not reset on the move")
	}
	// Staying put must not keep resetting the arrival clock.
	settled := moved.Add(Heartbeat + time.Second)
	if err := Touch(dir, Record{Session: "aaaa-1111", Den: "demo#second", Root: dir}, settled); err != nil {
		t.Fatal(err)
	}
	if InDen(dir, "demo#second", settled)[0].Since != arrived[0].Since {
		t.Error("a heartbeat in the same den reset the arrival time")
	}
}

// An agent outside any worktree, or one whose worktree was deleted underneath
// it, is still running. Neither may retire it: only silence does.
func TestLocationDoesNotDecideExistence(t *testing.T) {
	dir := t.TempDir()
	start := time.Now()
	if err := Touch(dir, Record{Session: "aaaa-1111", Den: "demo#gone",
		Root: filepath.Join(dir, "deleted-worktree")}, start); err != nil {
		t.Fatal(err)
	}
	if living := All(dir, start); len(living) != 1 {
		t.Fatalf("a live agent was pruned for a missing worktree: %d", len(living))
	}
	// Beat keeps it alive without knowing where it is, holding the last den.
	later := start.Add(Heartbeat + time.Second)
	if err := Beat(dir, "aaaa-1111", later); err != nil {
		t.Fatal(err)
	}
	held := All(dir, later)
	if len(held) != 1 || held[0].Den != "demo#gone" {
		t.Fatalf("Beat lost the agent or its den: %+v", held)
	}
	if held[0].Stale(later) {
		t.Error("Beat did not refresh the agent's liveness")
	}
}

// A den always shows a creature, and when somebody is working there it must be
// theirs. The ordering rule is the interesting part: presence beats recency.
func TestRepresentativePrefersALiveAgent(t *testing.T) {
	now := time.Now()
	live := Record{Session: "live", Seen: now.Add(-2 * time.Minute)}
	silent := Record{Session: "silent", Seen: now.Add(-1 * time.Minute)}

	if _, found := Representative(nil, now); found {
		t.Error("an empty den named a representative")
	}
	// Deliberately unsorted, and the stale one spoke more recently, so a naive
	// "take the newest" would pick the wrong agent.
	stale := Record{Session: "stale", Seen: now.Add(-StaleAfter - time.Minute)}
	chosen, found := Representative([]Record{stale, live}, now)
	if !found || chosen.Session != "live" {
		t.Errorf("a stale agent outranked a live one: %q", chosen.Session)
	}
	// Among agents in the same condition, the most recent wins.
	chosen, _ = Representative([]Record{live, silent}, now)
	if chosen.Session != "silent" {
		t.Errorf("among live agents the older one won: %q", chosen.Session)
	}
	// A den where everyone has gone quiet still gets a face.
	older := Record{Session: "older", Seen: now.Add(-StaleAfter - time.Hour)}
	chosen, found = Representative([]Record{older, stale}, now)
	if !found || chosen.Session != "stale" {
		t.Errorf("an all-stale den picked %q", chosen.Session)
	}
}

// Label is what a human reads in a list, so it must never come back blank.
func TestLabelFallsBackToTheSessionID(t *testing.T) {
	named := Record{Session: "aaaa-1111-bbbb", Name: "fix the flaky auth test"}
	if named.Label() != "fix the flaky auth test" {
		t.Errorf("named session labelled %q", named.Label())
	}
	if unnamed := (Record{Session: "aaaa-1111-bbbb"}).Label(); unnamed != "aaaa-111" {
		t.Errorf("unnamed session labelled %q", unnamed)
	}
}

// Only the status line payload carries a context window. A hook firing between two
// status line ticks passes zero, and must not overwrite what the status line knew.
func TestTouchKeepsAKnownContextWhenAHookReportsNone(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	record := Record{Session: "ctx-session", Den: "place:one#one", Context: 72}
	if err := Touch(dir, record, now); err != nil {
		t.Fatal(err)
	}

	// Past the heartbeat, so the write is not throttled, and with no context.
	later := now.Add(2 * Heartbeat)
	if err := Touch(dir, Record{Session: "ctx-session", Den: "place:one#one"}, later); err != nil {
		t.Fatal(err)
	}

	live := All(dir, later)
	if len(live) != 1 {
		t.Fatalf("register holds %d agents, want 1", len(live))
	}
	if live[0].Context != 72 {
		t.Errorf("context is %d after a hook reported none, want 72 preserved", live[0].Context)
	}
}

func TestContextETAFromBurnRate(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	if err := Touch(dir, Record{Session: "eta-session", Den: "place:one#one", Context: 40}, now); err != nil {
		t.Fatal(err)
	}
	later := now.Add(2 * time.Minute)
	if err := Touch(dir, Record{Session: "eta-session", Den: "place:one#one", Context: 60}, later); err != nil {
		t.Fatal(err)
	}
	live, ok := Get(dir, "eta-session")
	if !ok {
		t.Fatal("missing agent after context samples")
	}
	if live.PrevContext != 40 || live.Context != 60 {
		t.Fatalf("samples are prev=%d cur=%d, want 40 then 60", live.PrevContext, live.Context)
	}
	// 20% in 2 minutes => 10%/min => 40% remaining => ~4m
	eta := live.ContextETA()
	if eta != 4*time.Minute {
		t.Fatalf("ContextETA is %v, want 4m", eta)
	}
}

func TestTouchWritesChangedContextInsideHeartbeat(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	if err := Touch(dir, Record{Session: "hb-session", Den: "place:one#one", Context: 10}, now); err != nil {
		t.Fatal(err)
	}
	// Well inside Heartbeat (10s).
	soon := now.Add(2 * time.Second)
	if err := Touch(dir, Record{Session: "hb-session", Den: "place:one#one", Context: 25}, soon); err != nil {
		t.Fatal(err)
	}
	live, ok := Get(dir, "hb-session")
	if !ok {
		t.Fatal("missing agent")
	}
	if live.Context != 25 {
		t.Fatalf("context inside heartbeat stayed %d, want 25", live.Context)
	}
	if live.PrevContext != 10 {
		t.Fatalf("prev context is %d, want 10", live.PrevContext)
	}
}

func TestContextETAUnknownWithoutHistory(t *testing.T) {
	r := Record{Context: 70}
	if r.ContextETA() != 0 {
		t.Fatalf("single sample should not invent an ETA, got %v", r.ContextETA())
	}
}
