// Package den is the collection: every creature this machine has ever hatched.
//
// It is the only precious state in the project, and it is strictly additive.
// N worktrees writing it concurrently only ever add to a set, so there is no
// read-modify-write on a contested field and no conflict to resolve.
//
// The schema is versioned and every entry carries a stable key and a timestamp,
// so an opt-in sync layer can be added later without migrating anyone's file.
package den

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/TevvvB/termagitchi/internal/identity"
	"github.com/TevvvB/termagitchi/internal/lock"
)

const Version = 1

// Entry records the first time a creature was seen, and never changes after.
type Entry struct {
	Species   string    `json:"species"`
	Rarity    string    `json:"rarity"`
	Shiny     bool      `json:"shiny"`
	Branch    string    `json:"branch"`
	FirstSeen time.Time `json:"first_seen"`
}

type Den struct {
	Version int              `json:"version"`
	Entries map[string]Entry `json:"entries"`
}

func path(dir string) string {
	return filepath.Join(dir, "den.json")
}

// Load reads the collection. A missing or unreadable den is an empty one rather
// than an error, because nothing about rendering a pet depends on it.
func Load(dir string) Den {
	loaded := Den{Version: Version, Entries: map[string]Entry{}}
	data, err := os.ReadFile(path(dir))
	if err != nil {
		return loaded
	}
	if json.Unmarshal(data, &loaded) != nil || loaded.Entries == nil {
		return Den{Version: Version, Entries: map[string]Entry{}}
	}
	return loaded
}

// Record adds a creature if it is new and reports whether it was.
//
// It takes a lock because this is the one write that cannot simply be redone:
// two sessions hatching at the same moment would otherwise lose one entry.
func Record(dir string, pet identity.Pet, branch string, now time.Time) (bool, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	lockPath := filepath.Join(dir, "den.lock")
	if !lock.WaitAcquire(lockPath, 2*time.Minute) {
		return false, nil
	}
	defer os.Remove(lockPath)

	collection := Load(dir)
	key := pet.Key()
	if _, already := collection.Entries[key]; already {
		return false, nil
	}
	collection.Entries[key] = Entry{
		Species:   pet.Name,
		Rarity:    pet.Rarity.String(),
		Shiny:     pet.Shiny,
		Branch:    branch,
		FirstSeen: now,
	}
	encoded, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		return false, err
	}
	temporary := path(dir) + ".tmp"
	if err := os.WriteFile(temporary, append(encoded, '\n'), 0o644); err != nil {
		return false, err
	}
	return true, os.Rename(temporary, path(dir))
}

// Has reports whether a creature is already collected.
func (d Den) Has(pet identity.Pet) bool {
	_, found := d.Entries[pet.Key()]
	return found
}

// Progress is one rarity band's completion.
type Progress struct {
	Rarity identity.Rarity
	Have   int
	Total  int
}

// Completion counts distinct species per band, ignoring shiny duplicates so the
// denominator stays the size of the roster.
func (d Den) Completion() []Progress {
	totals := identity.CountByRarity()
	have := map[identity.Rarity]map[string]bool{}
	for _, entry := range d.Entries {
		for band := identity.Common; band <= identity.Mythic; band++ {
			if entry.Rarity != band.String() {
				continue
			}
			if have[band] == nil {
				have[band] = map[string]bool{}
			}
			have[band][entry.Species] = true
		}
	}
	var progress []Progress
	for band := identity.Common; band <= identity.Mythic; band++ {
		progress = append(progress, Progress{Rarity: band, Have: len(have[band]), Total: totals[band]})
	}
	return progress
}

// Distinct counts species, not entries, so a shiny does not inflate the total
// shown against a roster size that counts each creature once.
func (d Den) Distinct() int {
	seen := map[string]bool{}
	for _, entry := range d.Entries {
		seen[entry.Species] = true
	}
	return len(seen)
}

func (d Den) ShinyCount() int {
	count := 0
	for _, entry := range d.Entries {
		if entry.Shiny {
			count++
		}
	}
	return count
}

// Latest returns the most recently hatched entries, newest first.
func (d Den) Latest(limit int) []Entry {
	entries := make([]Entry, 0, len(d.Entries))
	for _, entry := range d.Entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(first, second int) bool {
		return entries[first].FirstSeen.After(entries[second].FirstSeen)
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}
