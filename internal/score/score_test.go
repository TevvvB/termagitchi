package score

import (
	"testing"

	"github.com/TevvvB/termagitchi/internal/config"
	"github.com/TevvvB/termagitchi/internal/state"
)

func TestScoringMatchesTheShellRules(t *testing.T) {
	settings := config.Default()
	cases := []struct {
		name   string
		state  state.State
		tests  string
		hearts int
	}{
		{"clean", state.State{}, "unknown", 5},
		{"one dirty file", state.State{Dirty: 1}, "unknown", 4},
		{"past the dirty threshold", state.State{Dirty: 16}, "unknown", 3},
		{"one unpushed commit", state.State{Unpushed: 1}, "unknown", 4},
		{"past the unpushed threshold", state.State{Unpushed: 6}, "unknown", 3},
		{"failing tests", state.State{}, "fail", 3},
		{"passing tests cost nothing", state.State{}, "pass", 5},
		{"behind never costs hearts", state.State{Behind: 500}, "unknown", 5},
		{"everything at once floors at zero", state.State{Dirty: 40, Unpushed: 40}, "fail", 0},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := Of(testCase.state, testCase.tests, settings).Hearts; got != testCase.hearts {
				t.Errorf("hearts = %d, want %d", got, testCase.hearts)
			}
		})
	}
}

// The migration penalty was the heaviest in the model and applied to everyone.
// It is now opt-in, so a repo with no migrations is never charged for them.
func TestMigrationPenaltyIsOffByDefault(t *testing.T) {
	split := state.State{Migrations: 3}
	if got := Of(split, "unknown", config.Default()).Hearts; got != 5 {
		t.Errorf("default config charged %d hearts for migrations, want 0 penalty", 5-got)
	}

	optedIn := config.Default()
	optedIn.Signals.Migrations.Enabled = true
	if got := Of(split, "unknown", optedIn).Hearts; got != 3 {
		t.Errorf("opted-in config gave %d hearts, want 3", got)
	}
}

// Scoring is data now, so a user can tune or disable any signal.
func TestDisablingASignalStopsItsPenalty(t *testing.T) {
	settings := config.Default()
	settings.Signals.Enabled = []string{"unpushed", "behind"}
	if got := Of(state.State{Dirty: 99}, "fail", settings).Hearts; got != 5 {
		t.Errorf("hearts = %d with dirty and tests disabled, want 5", got)
	}
}

func TestPenaltiesExplainThemselves(t *testing.T) {
	result := Of(state.State{Dirty: 20, Unpushed: 1}, "fail", config.Default())
	seen := map[string]int{}
	for _, penalty := range result.Penalties {
		seen[penalty.Signal] += penalty.Cost
	}
	if seen["uncommitted"] != 2 || seen["unpushed"] != 1 || seen["tests"] != 2 {
		t.Errorf("penalties = %v, want uncommitted:2 unpushed:1 tests:2", seen)
	}
}

func TestFaceDegradesWithHearts(t *testing.T) {
	previous := ""
	for hearts := 5; hearts >= 0; hearts-- {
		face := Face(hearts)
		if face == "" || face == previous {
			t.Errorf("Face(%d) = %q, want a distinct non-empty face", hearts, face)
		}
		previous = face
	}
}

func intPtr(value int) *int { return &value }

// The whole premise is that the face reflects the worktree, and external signals were the
// one user-extensible input that could not touch it: score never read State.External, so a
// signal reporting 40 clippy warnings left the pet delighted.
func TestExternalSignalsCostHearts(t *testing.T) {
	settings := config.Default()
	settings.Signals.External.Penalties = map[string]config.ExternalPenalty{
		"clippy":   {Over: intPtr(0), Cost: 2},
		"coverage": {Under: intPtr(80), Cost: 1},
	}

	clean := Of(state.State{External: map[string]int{"clippy": 0, "coverage": 95}}, "pass", settings)
	if clean.Hearts != settings.Score.Start {
		t.Errorf("signals within their bounds cost %d hearts", settings.Score.Start-clean.Hearts)
	}

	breached := Of(state.State{External: map[string]int{"clippy": 40, "coverage": 61}}, "pass", settings)
	if want := settings.Score.Start - 3; breached.Hearts != want {
		t.Errorf("hearts = %d, want %d after both rules fired", breached.Hearts, want)
	}
	// The card explains itself from these, so each breach has to name its own signal.
	named := map[string]bool{}
	for _, penalty := range breached.Penalties {
		named[penalty.Signal] = true
	}
	if !named["clippy"] || !named["coverage"] {
		t.Errorf("penalties %+v do not name both signals", breached.Penalties)
	}
}

// A probe that did not run reports nothing, which is not the same as reporting zero. Left
// unguarded, every "under" rule fires on every worktree where the signal is absent.
func TestUnreportedExternalSignalIsNotZero(t *testing.T) {
	settings := config.Default()
	settings.Signals.External.Penalties = map[string]config.ExternalPenalty{
		"coverage": {Under: intPtr(80), Cost: 2},
	}

	result := Of(state.State{External: map[string]int{}}, "pass", settings)
	if result.Hearts != settings.Score.Start {
		t.Errorf("an absent coverage signal cost %d hearts", settings.Score.Start-result.Hearts)
	}
}
