package den

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/TevvvB/termagitchi/internal/identity"
)

func TestRecordIsAdditiveAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	pet := identity.For("feat/checkout-flow")
	now := time.Now()

	added, err := Record(dir, pet, "feat/checkout-flow", now)
	if err != nil || !added {
		t.Fatalf("first Record = %v/%v, want true/nil", added, err)
	}
	added, err = Record(dir, pet, "some/other-branch", now.Add(time.Hour))
	if err != nil || added {
		t.Fatalf("second Record = %v/%v, want false/nil", added, err)
	}

	collection := Load(dir)
	if len(collection.Entries) != 1 {
		t.Fatalf("den has %d entries, want 1", len(collection.Entries))
	}
	// First sighting is the record; a later branch must not overwrite it.
	if collection.Entries[pet.Key()].Branch != "feat/checkout-flow" {
		t.Error("a later sighting overwrote the original branch")
	}
}

// The whole point is N worktrees running at once. Concurrent hatches must not
// lose entries to a read-modify-write race.
func TestConcurrentRecordsDoNotLoseEntries(t *testing.T) {
	dir := t.TempDir()
	branches := []string{
		"feat/checkout-flow", "fix/session-leak", "spike/wasm-build",
		"chore/bump-deps", "docs/install-rewrite", "refactor/auth-guard",
		"perf/query-cache", "test/e2e-flake",
	}
	expected := map[string]bool{}
	for _, branch := range branches {
		expected[identity.For(branch).Key()] = true
	}

	var waiting sync.WaitGroup
	for _, branch := range branches {
		waiting.Add(1)
		go func(branch string) {
			defer waiting.Done()
			Record(dir, identity.For(branch), branch, time.Now())
		}(branch)
	}
	waiting.Wait()

	collection := Load(dir)
	for key := range expected {
		if _, found := collection.Entries[key]; !found {
			t.Errorf("entry %q was lost to a concurrent write", key)
		}
	}
}

func TestShiniesAreCollectedSeparately(t *testing.T) {
	dir := t.TempDir()
	base := identity.Species{Name: "koi", Prefix: "=(", Suffix: ")=", Rarity: identity.Rare}
	plain := identity.Pet{Species: base, Color: 81}
	shiny := identity.Pet{Species: base, Color: 81, Shiny: true}

	Record(dir, plain, "a", time.Now())
	added, _ := Record(dir, shiny, "b", time.Now())
	if !added {
		t.Fatal("a shiny of an owned species was rejected as a duplicate")
	}
	if got := Load(dir).ShinyCount(); got != 1 {
		t.Errorf("ShinyCount = %d, want 1", got)
	}
}

// Completion counts distinct species, so owning both a plain and a shiny koi is
// still one of eight rares, not two.
func TestCompletionCountsDistinctSpecies(t *testing.T) {
	dir := t.TempDir()
	base := identity.Species{Name: "koi", Prefix: "=(", Suffix: ")=", Rarity: identity.Rare}
	Record(dir, identity.Pet{Species: base}, "a", time.Now())
	Record(dir, identity.Pet{Species: base, Shiny: true}, "b", time.Now())

	for _, band := range Load(dir).Completion() {
		if band.Rarity == identity.Rare && band.Have != 1 {
			t.Errorf("rare completion = %d, want 1", band.Have)
		}
	}
}

// The den is the only precious state, so a corrupt file must degrade to empty
// rather than take the status line down with it.
func TestCorruptDenLoadsAsEmpty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "den.json"), []byte("{not json"), 0o644)
	if got := len(Load(dir).Entries); got != 0 {
		t.Errorf("corrupt den loaded %d entries, want 0", got)
	}
}

// A versioned schema with stable keys and timestamps is what lets an opt-in sync
// layer arrive later without migrating anyone's file.
func TestPersistedShapeIsSyncReady(t *testing.T) {
	dir := t.TempDir()
	Record(dir, identity.For("feat/checkout-flow"), "feat/checkout-flow", time.Now())

	data, err := os.ReadFile(filepath.Join(dir, "den.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Version int `json:"version"`
		Entries map[string]struct {
			Species   string `json:"species"`
			Rarity    string `json:"rarity"`
			FirstSeen string `json:"first_seen"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if raw.Version != Version {
		t.Errorf("version = %d, want %d", raw.Version, Version)
	}
	for key, entry := range raw.Entries {
		if key == "" || entry.Species == "" || entry.Rarity == "" {
			t.Errorf("entry %q is missing a stable identifier: %+v", key, entry)
		}
		if _, err := time.Parse(time.RFC3339, entry.FirstSeen); err != nil {
			t.Errorf("first_seen %q is not RFC3339: %v", entry.FirstSeen, err)
		}
	}
}
