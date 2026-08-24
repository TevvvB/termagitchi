package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TevvvB/parallel-harness-pets/internal/agents"
	"github.com/TevvvB/parallel-harness-pets/internal/gitrepo"
	"github.com/TevvvB/parallel-harness-pets/internal/identity"
	"github.com/TevvvB/parallel-harness-pets/internal/state"
)

// worktree fakes just enough of a checkout for gitrepo.Locate, which reads
// .git/HEAD and nothing else.
func worktree(t *testing.T, branch string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	head := []byte("ref: refs/heads/" + branch + "\n")
	if err := os.WriteFile(filepath.Join(root, ".git", "HEAD"), head, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// fixture returns a captured payload with its recorded worktree path swapped
// for a real one. Only the path is rewritten: the shape, and every field the
// code does not read, stay exactly as the harness sent them.
//
// The replacement is escaped as a JSON string rather than spliced in raw. A
// Windows temp directory is C:\Users\..., and every backslash in it is an
// invalid JSON escape, so splicing produces a payload that silently fails to
// parse and a test that reports every field missing.
func fixture(t *testing.T, name, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	quoted := string(encoded[1 : len(encoded)-1])
	return strings.ReplaceAll(string(raw), "/home/user/code/demo/.worktrees/feat-example", quoted)
}

// Replaying real captured bytes, rather than a payload we invented, is the only
// way to catch the class of bug that already shipped here once: hand-written
// fixtures encode what the author believes the harness sends, so a test built
// on one agrees with the mistake.
//
// The assertion is on the cache, not the return value. A hook that decodes
// perfectly and writes the wrong verdict, or the right verdict with a timestamp
// that never expires, satisfies any test that only checks what came back.
func TestRecordFromCapturedPayloadWritesAndExpires(t *testing.T) {
	root := worktree(t, "feat-example")
	cache := t.TempDir()
	now := time.Now()

	result, ok := recordFrom(strings.NewReader(fixture(t, "posttooluse.json", root)), cache, now)
	if !ok {
		t.Fatal("a real PostToolUse payload was not recorded at all")
	}
	if result != "pass" {
		t.Errorf("recorded %q from a passing `go test` run, want pass", result)
	}

	// What the hook exists to produce: a verdict another process can read back.
	key := repoKey(t, root)
	if stored := state.ReadTests(cache, key, now); stored != "pass" {
		t.Errorf("cache holds %q, want pass", stored)
	}
	// And which stops being trusted, so a green run cannot vouch for code
	// somebody has been editing for hours.
	if stale := state.ReadTests(cache, key, now.Add(3*time.Hour)); stale != "unknown" {
		t.Errorf("a three hour old verdict still read as %q", stale)
	}
}

// The status line payload is captured too, because it carries the two fields
// the whole den and pet model is keyed on. If a harness ever renames either,
// this is what notices.
func TestCapturedStatuslinePayloadStillCarriesItsKeys(t *testing.T) {
	root := worktree(t, "feat-example")
	payload := parsePayload(strings.NewReader(fixture(t, "statusline.json", root)), time.Second)

	if payload.SessionID == "" {
		t.Error("no session_id: pets could not be keyed to an agent")
	}
	if payload.Workspace.GitWorktree == "" {
		t.Error("no workspace.git_worktree: dens would fall back to guessing")
	}
	if payload.Workspace.Repo.Name == "" {
		t.Error("no workspace.repo.name: every repo's worktrees would share a den")
	}
	if payload.Model.DisplayName == "" {
		t.Error("no model.display_name")
	}
}

func repoKey(t *testing.T, root string) string {
	t.Helper()
	located, found := gitrepo.Locate(root)
	if !found {
		t.Fatalf("could not locate the worktree at %s", root)
	}
	return located.Key()
}

// A tool use that is not a test run still has to register the agent: for a harness with
// hooks and no status line, this hook is the only per-turn signal that it is alive.
func TestRecordFromRegistersTheAgentEvenWhenTheCommandIsNotATestRun(t *testing.T) {
	root := worktree(t, "feat-example")
	cache := t.TempDir()
	now := time.Now()

	// The captured payload with only the command swapped, so verdict.Of gives up on it.
	payload := strings.ReplaceAll(fixture(t, "posttooluse.json", root), "go test ./...", "ls -la")
	if _, ok := recordFrom(strings.NewReader(payload), cache, now); ok {
		t.Fatal("`ls -la` was taken for a test verdict")
	}

	live := agents.All(cache, now)
	if len(live) != 1 {
		t.Fatalf("register holds %d agents, want the one that just used a tool", len(live))
	}
	if live[0].Session != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("registered session %q, not the one in the payload", live[0].Session)
	}
	if live[0].Den == "" {
		t.Error("agent registered with no den, so no listing can group it")
	}
}

// Codex is the reason parsePayload must not treat a decode error as fatal.
//
// Its payload sends model as a plain string where Claude Code sends an object,
// so unmarshalling it always returns an UnmarshalTypeError. Go fills in every
// other field regardless, which is the only reason Codex works at all. Add the
// error check that line looks like it is missing and every Codex hook stops
// registering agents, with the whole Claude Code suite still green.
//
// Codex also sends no workspace object, so the den has to come from git. A
// registerAgent that trusted the payload's worktree would put every checkout
// in one shared den, and again only Codex would notice.
func TestCapturedCodexPayloadRegistersIntoTheGitDerivedDen(t *testing.T) {
	root := worktree(t, "feat-example")
	cache := t.TempDir()
	now := time.Now()

	payload := parsePayload(strings.NewReader(fixture(t, "codex-sessionstart.json", root)), time.Second)
	if payload.SessionID == "" {
		t.Fatal("no session_id survived the decode: every Codex hook is anonymous")
	}
	if payload.Workspace.GitWorktree != "" {
		t.Fatal("fixture has a workspace object, so it no longer exercises the git fallback")
	}

	located, found := gitrepo.Locate(root)
	if !found {
		t.Fatalf("could not locate the worktree at %s", root)
	}
	den := registerAgent(cache, located, payload.agent(), now)
	if want := identity.DenKey(located.Project(), located.Worktree()); den != want {
		t.Errorf("den %q, want the git-derived %q", den, want)
	}

	residents := agents.InDen(cache, den, now)
	if len(residents) != 1 {
		t.Fatalf("den holds %d agents after one Codex hook, want 1", len(residents))
	}
	if residents[0].Session != payload.SessionID {
		t.Errorf("den holds session %q, want %q", residents[0].Session, payload.SessionID)
	}
}
