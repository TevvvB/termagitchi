package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/TevvvB/parallel-harness-pets/internal/agents"
	"github.com/TevvvB/parallel-harness-pets/internal/config"
	"github.com/TevvvB/parallel-harness-pets/internal/den"
	"github.com/TevvvB/parallel-harness-pets/internal/gitrepo"
	"github.com/TevvvB/parallel-harness-pets/internal/identity"
	"github.com/TevvvB/parallel-harness-pets/internal/release"
	"github.com/TevvvB/parallel-harness-pets/internal/render"
	"github.com/TevvvB/parallel-harness-pets/internal/score"
	"github.com/TevvvB/parallel-harness-pets/internal/signal"
	"github.com/TevvvB/parallel-harness-pets/internal/state"
	"github.com/TevvvB/parallel-harness-pets/internal/visits"
)

// updateNotice checks at most once a day, and only ever from here: the three
// commands a person types. Hooks and the render path stay offline.
func updateNotice(settings config.Config) string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	latest := release.Check(config.StateDir(), version, settings.Update.Check, time.Now())
	return render.UpdateNotice(latest, release.Command(executable))
}

func denCommand() {
	cacheDir := config.StateDir()
	// Cities are collected the same way creatures are, so the den needs both
	// halves. Which places have been worked in comes out of the visit log,
	// deduped and resolved back through the same hash that assigned them.
	seen := map[string]bool{}
	var visited []identity.Place
	for _, key := range visits.Dens(cacheDir) {
		place := identity.PlaceForKey(key)
		if seen[place.Code] {
			continue
		}
		seen[place.Code] = true
		visited = append(visited, place)
	}
	sort.Slice(visited, func(first, second int) bool {
		return visited[first].Code < visited[second].Code
	})
	fmt.Print(render.Den(den.Load(cacheDir), visited))
	fmt.Print(updateNotice(config.Load()))
}

// partyCommand shows every worktree the cache knows about that still exists.
func partyCommand(args []string) {
	settings := config.Load()
	cacheDir := config.StateDir()
	now := time.Now()

	// Read the register once and group, rather than re-scanning it per worktree.
	byDen := map[string][]agents.Record{}
	for _, record := range agents.All(cacheDir, now) {
		byDen[record.Den] = append(byDen[record.Den], record)
	}

	var views []render.View
	for _, current := range state.All(cacheDir) {
		repo, found := gitrepo.Locate(current.Root)
		if !found {
			continue
		}
		if current.Stale(now) {
			refreshInBackground(repo.Root)
		}
		tests := state.ReadTests(cacheDir, repo.Key(), now)
		denKey := identity.DenKey(repo.Project(), repo.Worktree())
		// A den always shows a creature, but it belongs to whoever is working
		// there when anyone is. Only an empty den falls back to the branch.
		residents := byDen[denKey]
		petKey := repo.Branch
		if representative, occupied := agents.Representative(residents, now); occupied {
			petKey = representative.Session
		}
		views = append(views, render.View{
			Pet:       identity.For(petKey),
			Place:     identity.PlaceFor(repo.Project(), repo.Worktree()),
			Den:       denKey,
			Residents: residents,
			Branch:    repo.Branch,
			Root:      repo.Root,
			State:     current,
			Tests:     tests,
			Score:     score.Of(current, tests, settings),
			HasState:  true,
		})
	}
	showAll := false
	for _, arg := range args {
		if arg == "--all" {
			showAll = true
		}
	}
	fmt.Print(render.Party(views, settings, showAll, now))
	fmt.Print(updateNotice(settings))
}

// hatchMarker records that a worktree has already been greeted, so the hatch
// fires exactly once per worktree rather than once per session.
func hatchMarker(cacheDir, key string) string {
	return filepath.Join(cacheDir, key+".hatched")
}

// hatchCommand runs on SessionStart. It is the only animated moment in the tool.
func hatchCommand() {
	settings := config.Load()
	payload := readPayload()
	repo, found := gitrepo.Locate(payload.directory())
	if !found {
		return
	}
	cacheDir := config.StateDir()
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return
	}
	// Ahead of the marker check, which returns for any worktree that hatched before.
	registerAgent(cacheDir, repo, payload.agent(), time.Now())
	// pets party lists worktrees that have state, and joins agents onto them, so
	// registering without probing leaves a live session in the register and absent
	// from the listing. Session start is also when fresh state is most wanted, and
	// probe self-throttles behind a lock.
	probe(repo.Root, settings)
	marker := hatchMarker(cacheDir, repo.Key())
	if _, err := os.Stat(marker); err == nil {
		return
	}
	if err := os.WriteFile(marker, []byte(time.Now().Format(time.RFC3339)+"\n"), 0o644); err != nil {
		return
	}

	// Recording happens before rendering so the position shown is the real one.
	// The hatch itself is free, but the den entry has to be earned, or a
	// checkout -b loop farms the collection.
	recordIfEarned(repo, settings, cacheDir)

	pet := identity.For(repo.Branch)
	collection := den.Load(cacheDir)
	fmt.Print(render.Hatch(pet, collection.Distinct(), len(identity.All()), collection.Has(pet)))
}

// recordIfEarned adds a creature to the den once its worktree has a commit.
//
// The already-collected check comes first because this runs on every probe, and
// Earned costs several git subprocesses that are pure waste for a creature the
// den has held since last week.
func recordIfEarned(repo gitrepo.Repo, settings config.Config, cacheDir string) bool {
	pet := identity.For(repo.Branch)
	if den.Load(cacheDir).Has(pet) {
		return false
	}
	if !signal.Earned(repo, settings) {
		return false
	}
	added, _ := den.Record(cacheDir, pet, repo.Branch, time.Now())
	return added
}
