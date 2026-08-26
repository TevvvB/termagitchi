// Package config holds every threshold and penalty the shell version hardcoded.
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Branch struct {
	// Empty means detect from origin/HEAD rather than assuming main.
	Default string `toml:"default"`
}

type Score struct {
	Start          int `toml:"start"`
	DirtyAny       int `toml:"dirty_any"`
	DirtyMany      int `toml:"dirty_many"`
	DirtyManyAt    int `toml:"dirty_many_at"`
	UnpushedAny    int `toml:"unpushed_any"`
	UnpushedMany   int `toml:"unpushed_many"`
	UnpushedManyAt int `toml:"unpushed_many_at"`
	TestsFailing   int `toml:"tests_failing"`
}

type Display struct {
	BehindShownPast int `toml:"behind_shown_past"`
	BranchLabelMax  int `toml:"branch_label_max"`
	// PartyLimit caps the party view. Sorted worst-first, so the rows that get
	// hidden are the healthy ones nobody needs to read.
	PartyLimit int `toml:"party_limit"`
}

// Migrations was the alembic check. It is off by default because it costs a
// large penalty for a signal most projects cannot produce at all.
type Migrations struct {
	Enabled bool     `toml:"enabled"`
	Penalty int      `toml:"penalty"`
	Paths   []string `toml:"paths"`
}

type External struct {
	Dir       string                     `toml:"dir"`
	TimeoutMs int                        `toml:"timeout_ms"`
	Penalties map[string]ExternalPenalty `toml:"penalties"`
}

// ExternalPenalty makes one user signal count against the score.
//
// Two bounds because half of what people measure is bad going up (warnings, TODOs,
// drifted resources) and half is bad going down (coverage, checks passing). Pointers so
// that "over = 0", the most useful rule there is, is distinguishable from unset.
type ExternalPenalty struct {
	Over  *int `toml:"over"`
	Under *int `toml:"under"`
	Cost  int  `toml:"cost"`
}

// Triggered reports whether a reported value breaches this rule.
func (p ExternalPenalty) Triggered(value int) bool {
	if p.Over != nil && value > *p.Over {
		return true
	}
	return p.Under != nil && value < *p.Under
}

// Update controls the once-a-day check for a newer release. It runs only from
// commands a person types, never from a hook or the render path.
type Update struct {
	Check bool `toml:"check"`
}

type Signals struct {
	Enabled    []string   `toml:"enabled"`
	Migrations Migrations `toml:"migrations"`
	External   External   `toml:"external"`
}

type Config struct {
	Branch  Branch  `toml:"branch"`
	Update  Update  `toml:"update"`
	Score   Score   `toml:"score"`
	Display Display `toml:"display"`
	Signals Signals `toml:"signals"`
}

// Default reproduces the shell version's behaviour exactly, minus the
// migration check, which becomes opt-in.
func Default() Config {
	return Config{
		Score: Score{
			Start:          5,
			DirtyAny:       1,
			DirtyMany:      1,
			DirtyManyAt:    15,
			UnpushedAny:    1,
			UnpushedMany:   1,
			UnpushedManyAt: 5,
			TestsFailing:   2,
		},
		Display: Display{BehindShownPast: 40, BranchLabelMax: 26, PartyLimit: 12},
		Update:  Update{Check: true},
		Signals: Signals{
			Enabled: []string{"dirty", "unpushed", "behind", "tests"},
			Migrations: Migrations{
				Penalty: 2,
				Paths:   []string{"alembic/versions", "migrations/versions"},
			},
			External: External{TimeoutMs: 800},
		},
	}
}

// Dir is where config and packs live, honouring XDG_CONFIG_HOME.
func Dir() string {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "pets")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".pets"
	}
	return filepath.Join(home, ".config", "pets")
}

// StateDir holds the disposable cache and, later, the den.
func StateDir() string {
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, "pets")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".pets-state"
	}
	return filepath.Join(home, ".local", "state", "pets")
}

// Load layers the user's config over the defaults. A missing file is normal;
// a malformed one falls back to defaults rather than leaving the status line broken.
func Load() Config {
	loaded := Default()
	path := filepath.Join(Dir(), "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return loaded
	}
	if _, err := toml.Decode(string(data), &loaded); err != nil {
		return Default()
	}
	return loaded
}

func (c Config) SignalEnabled(name string) bool {
	for _, enabled := range c.Signals.Enabled {
		if enabled == name {
			return true
		}
	}
	return false
}
