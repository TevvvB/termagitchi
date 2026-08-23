// Command pets renders a per-worktree creature and the branch hygiene behind its mood.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/TevvvB/parallel-harness-pets/internal/agents"
	"github.com/TevvvB/parallel-harness-pets/internal/config"
	"github.com/TevvvB/parallel-harness-pets/internal/gitrepo"
	"github.com/TevvvB/parallel-harness-pets/internal/identity"
	"github.com/TevvvB/parallel-harness-pets/internal/lock"
	"github.com/TevvvB/parallel-harness-pets/internal/render"
	"github.com/TevvvB/parallel-harness-pets/internal/score"
	"github.com/TevvvB/parallel-harness-pets/internal/signal"
	"github.com/TevvvB/parallel-harness-pets/internal/state"
	"github.com/TevvvB/parallel-harness-pets/internal/verdict"
	"github.com/TevvvB/parallel-harness-pets/internal/visits"
)

var version = "dev"

const usage = `pets - a creature for every worktree

  pets render [--format=statusline|tmux|title|json]  render for a surface
  pets party [--all]                                 every live worktree at once
  pets den                                           the collection
  pets card [path]                                   full readout
  pets probe <path>                                  refresh one worktree's cache
  pets hatch                                         session-start hook, reads JSON on stdin
  pets record                                        tool-use hook, reads JSON on stdin
  pets quip                                          stop hook, reads JSON on stdin
  pets install [--harness=claude|codex|tmux|shell|all]
  pets uninstall
  pets version
`

func main() {
	os.Exit(dispatch(os.Args[1:]))
}

// dispatch routes a command and returns the process exit code.
//
// Asking for help is a success and prints to stdout; an unknown command is a
// failure and prints to stderr. Conflating the two makes pets --help exit 1,
// which passes unnoticed interactively and fails every script that runs it.
func dispatch(args []string) int {
	if len(args) == 0 {
		fmt.Print(usage)
		return 0
	}
	switch args[0] {
	case "help", "--help", "-h":
		fmt.Print(usage)
		return 0
	case "render":
		renderCommand(args[1:])
	case "party":
		partyCommand(args[1:])
	case "den":
		denCommand()
	case "card":
		cardCommand(args[1:])
	case "probe":
		probeCommand(args[1:])
	case "hatch":
		hatchCommand()
	case "record":
		recordCommand()
	case "quip":
		quipCommand()
	case "install", "uninstall":
		installCommand(args[0], args[1:])
	case "version", "--version", "-v":
		fmt.Println(version)
	default:
		fmt.Fprintf(os.Stderr, "pets: unknown command %q\n\n%s", args[0], usage)
		return 1
	}
	return 0
}

// hookPayload is the shape both Claude Code and Codex pipe into a hook.
type hookPayload struct {
	// SessionID identifies one running agent. It is what makes a pet belong to
	// an agent rather than to the directory the agent happens to be in.
	SessionID string `json:"session_id"`
	// SessionName is the harness's human label for this agent. It is what turns
	// a list of pets into something readable instead of a column of UUIDs.
	SessionName string `json:"session_name"`
	Cwd         string `json:"cwd"`
	Workspace   struct {
		CurrentDir string `json:"current_dir"`
		// GitWorktree and Repo name the den. The harness has already resolved
		// both, so the render path never has to ask git for them.
		GitWorktree string `json:"git_worktree"`
		Repo        struct {
			Owner string `json:"owner"`
			Name  string `json:"name"`
		} `json:"repo"`
	} `json:"workspace"`
	Model struct {
		DisplayName string `json:"display_name"`
	} `json:"model"`
	// The only genuinely per-agent signal here: mood comes from the worktree and is shared.
	ContextWindow struct {
		UsedPercentage int `json:"used_percentage"`
	} `json:"context_window"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`
}

// agent is what a surface knows about the caller: which agent is asking, and
// which den it is asking from. Every field is optional, because tmux, a shell
// prompt and `pets card` supply none of them.
type agent struct {
	SessionID string
	Name      string
	Repo      string
	Worktree  string
	Model     string
	Context   int
}

func (p hookPayload) agent() agent {
	return agent{
		SessionID: p.SessionID,
		Name:      p.SessionName,
		Repo:      p.Workspace.Repo.Name,
		Worktree:  p.Workspace.GitWorktree,
		Model:     p.Model.DisplayName,
		Context:   p.ContextWindow.UsedPercentage,
	}
}

// stdinDeadline bounds how long a surface may take to hand over its payload.
//
// A harness that pipes JSON writes it immediately and closes, so this is never
// reached. A surface that does not, such as tmux running us through #(...), can
// hand us an inherited stdin that never closes, and an unbounded read there
// hangs the status bar forever.
const stdinDeadline = 150 * time.Millisecond

// readPayload parses harness JSON from stdin, tolerating an absent, empty, or
// never-closing one.
func readPayload() hookPayload {
	var payload hookPayload
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return payload
	}
	return parsePayload(os.Stdin, stdinDeadline)
}

// parsePayload is readPayload's testable core: it gives up rather than blocking
// forever on a reader that never reaches EOF.
func parsePayload(source io.Reader, deadline time.Duration) hookPayload {
	var payload hookPayload
	received := make(chan []byte, 1)
	go func() {
		data, err := io.ReadAll(source)
		if err != nil {
			received <- nil
			return
		}
		received <- data
	}()
	select {
	case data := <-received:
		if len(data) > 0 {
			json.Unmarshal(data, &payload)
		}
	case <-time.After(deadline):
	}
	return payload
}

func (p hookPayload) directory() string {
	for _, candidate := range []string{p.Workspace.CurrentDir, p.Cwd} {
		if candidate != "" {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
		}
	}
	working, err := os.Getwd()
	if err != nil {
		return "."
	}
	return working
}

// registerAgent pins an agent to its den and returns the den key; every payload path calls it, not only the status line.
func registerAgent(cacheDir string, repo gitrepo.Repo, who agent, now time.Time) string {
	worktree := who.Worktree
	if worktree == "" {
		worktree = repo.Worktree()
	}
	project := who.Repo
	if project == "" {
		project = repo.Project()
	}
	den := identity.DenKey(project, worktree)
	agents.Touch(cacheDir, agents.Record{
		Session: who.SessionID,
		Den:     den,
		Root:    repo.Root,
		Branch:  repo.Branch,
		Name:    who.Name,
		Context: who.Context,
	}, now)
	// Touch forgets an agent the moment it moves; the visit log is the half that does not.
	visits.Record(cacheDir, den, who.SessionID, now)
	return den
}

// build assembles a view from cache only. It never runs git, because this is the
// path that executes once a second in every open session.
func build(directory string, who agent, settings config.Config) (render.View, bool) {
	repo, found := gitrepo.Locate(directory)
	if !found {
		return render.View{}, false
	}
	now := time.Now()
	cacheDir := config.StateDir()
	current, hasState := state.Read(cacheDir, repo.Key())
	tests := state.ReadTests(cacheDir, repo.Key(), now)

	if !hasState || current.Stale(now) {
		refreshInBackground(repo.Root)
	}

	// The pet belongs to the agent. Surfaces with no session to key on — tmux, a
	// shell prompt, `pets card` — fall back to the branch, which is what the pet
	// meant before agents were modelled at all.
	petKey := who.SessionID
	if petKey == "" {
		petKey = repo.Branch
	}
	den := registerAgent(cacheDir, repo, who, now)

	view := render.View{
		Pet:      identity.For(petKey),
		Place:    identity.PlaceForKey(den),
		Den:      den,
		Session:  who.SessionID,
		Branch:   repo.Branch,
		Root:     repo.Root,
		State:    current,
		Tests:    tests,
		Model:    who.Model,
		HasState: hasState,
	}
	if hasState {
		view.Score = score.Of(current, tests, settings)
	}
	return view, true
}

// refreshInBackground detaches a probe so the render never waits on git.
func refreshInBackground(root string) {
	executable, err := os.Executable()
	if err != nil {
		return
	}
	command := exec.Command(executable, "probe", root)
	command.Stdout, command.Stderr = io.Discard, io.Discard
	if command.Start() == nil {
		go command.Wait()
	}
}

func flagValue(args []string, name, fallback string) string {
	for _, arg := range args {
		if value, found := stringsCutPrefix(arg, "--"+name+"="); found {
			return value
		}
	}
	return fallback
}

func stringsCutPrefix(text, prefix string) (string, bool) {
	if len(text) >= len(prefix) && text[:len(prefix)] == prefix {
		return text[len(prefix):], true
	}
	return "", false
}

func renderCommand(args []string) {
	settings := config.Load()
	payload := readPayload()
	// A surface with no payload can name the worktree itself. tmux passes
	// #{pane_current_path} this way.
	directory := flagValue(args, "cwd", "")
	if directory == "" {
		directory = payload.directory()
	}
	view, found := build(directory, payload.agent(), settings)
	format := flagValue(args, "format", "statusline")
	if !found {
		// Outside a worktree an agent is still running, so it keeps breathing in
		// whichever den it was last seen in. Without this, stepping into /tmp
		// made a live session vanish from every listing 90 seconds later.
		agents.Beat(config.StateDir(), payload.SessionID, time.Now())
		if format == "json" {
			fmt.Println(`{"ready":false}`)
		}
		return
	}
	switch format {
	case "tmux":
		fmt.Println(render.Tmux(view, settings))
	case "title":
		fmt.Println(render.Title(view, settings))
	case "json":
		encoded, err := render.JSON(view, settings)
		if err != nil {
			os.Exit(1)
		}
		fmt.Println(encoded)
	default:
		fmt.Println(render.Statusline(view, settings))
	}
}

func cardCommand(args []string) {
	settings := config.Load()
	directory := ""
	for _, arg := range args {
		if len(arg) > 0 && arg[0] != '-' {
			directory = arg
		}
	}
	if directory == "" {
		directory, _ = os.Getwd()
	}
	repo, found := gitrepo.Locate(directory)
	if !found {
		fmt.Printf("(-.-) no git worktree at %s\n", directory)
		os.Exit(1)
	}
	// The card is on demand, so it can afford to be current rather than cached.
	probe(repo.Root, settings)
	view, ok := build(repo.Root, agent{}, settings)
	if !ok {
		os.Exit(1)
	}
	// The card is a den's readout, so it lists everyone in the den rather than
	// guessing at a single pet it has no session to identify.
	now := time.Now()
	view.Residents = agents.InDen(config.StateDir(), view.Den, now)
	// The card runs from a shell with no session of its own, so without this it
	// shows a branch-derived creature nobody in the den is actually using. Session
	// stays empty on purpose: it means "which agent is asking", and nobody is.
	if representative, occupied := agents.Representative(view.Residents, now); occupied {
		view.Pet = identity.For(representative.Session)
	}
	view.Visitors = visits.ForDen(config.StateDir(), view.Den)
	fmt.Print(render.Card(view, settings))
	fmt.Print(updateNotice(settings))
}

func probeCommand(args []string) {
	if len(args) == 0 {
		os.Exit(0)
	}
	probe(args[0], config.Load())
}

// probe holds a directory lock so several sessions refreshing at once do not
// stampede the same worktree.
func probe(root string, settings config.Config) {
	repo, found := gitrepo.Locate(root)
	if !found {
		return
	}
	cacheDir := config.StateDir()
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return
	}
	lockPath := filepath.Join(cacheDir, repo.Key()+".lock")
	if !lock.TryAcquire(lockPath, 5*time.Minute) {
		return
	}
	defer os.Remove(lockPath)
	state.Write(cacheDir, repo.Key(), signal.Collect(repo, settings, time.Now()))
	// A branch earns its den entry on its first commit, which may be long after
	// the hatch, so the probe is where that gets noticed.
	recordIfEarned(repo, settings, cacheDir)
}

func recordCommand() {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return
	}
	recordFrom(os.Stdin, config.StateDir(), time.Now())
}

// recordFrom is recordCommand's testable core.
//
// It takes the bytes a harness actually pipes in and returns the verdict it
// wrote, so a test can assert what landed in the cache rather than only what
// the parser returned. The hook's product is a file with an expiry, and a
// function that decodes correctly while writing the wrong thing would satisfy
// any test that only checks a return value.
func recordFrom(source io.Reader, cacheDir string, now time.Time) (string, bool) {
	payload := parsePayload(source, stdinDeadline)
	repo, found := gitrepo.Locate(payload.directory())
	if !found {
		return "", false
	}
	// Ahead of the verdict check, which bails on any command that is not a test run.
	registerAgent(cacheDir, repo, payload.agent(), now)
	result, ok := verdict.Of(payload.ToolInput.Command, responseText(payload.ToolResponse))
	if !ok {
		return "", false
	}
	if err := state.WriteTests(cacheDir, repo.Key(), result, now); err != nil {
		return "", false
	}
	return result, true
}

// responseText decodes what a runner actually printed.
//
// Casting the raw message to a string yields JSON source: quotes included and
// newlines still written as backslash-n. Substring checks survive that, but
// anything anchored to a line never matches, so the text has to be decoded.
// Harnesses send either a bare string or an object carrying stdout and stderr.
func responseText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var fields map[string]any
	if json.Unmarshal(raw, &fields) == nil {
		var parts []string
		for _, key := range []string{"stdout", "stderr", "output", "content", "result"} {
			if value, isText := fields[key].(string); isText && value != "" {
				parts = append(parts, value)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}
	return string(raw)
}

func quipCommand() {
	settings := config.Load()
	payload := readPayload()
	repo, found := gitrepo.Locate(payload.directory())
	if !found {
		// As on the render path: an agent outside a worktree is still running.
		agents.Beat(config.StateDir(), payload.SessionID, time.Now())
		return
	}
	probe(repo.Root, settings)
	view, ok := build(repo.Root, payload.agent(), settings)
	if !ok {
		return
	}
	message := fmt.Sprintf("%s %s: %s", view.Body(), view.Pet.Name, quipFor(view))
	encoded, err := json.Marshal(map[string]string{"systemMessage": message})
	if err != nil {
		return
	}
	fmt.Println(string(encoded))
}

// quipFor names the worst thing it can see, so the line is always the useful one.
func quipFor(view render.View) string {
	switch {
	case view.State.Migrations > 1:
		return fmt.Sprintf("%d migration heads. that one never fixes itself.", view.State.Migrations)
	case view.Tests == "fail":
		return "tests are red. i saw it."
	case view.State.Unpushed > 5:
		return fmt.Sprintf("%d unpushed. this branch only exists on your laptop.", view.State.Unpushed)
	case view.State.Dirty > 15:
		return fmt.Sprintf("%d files uncommitted. that is a lot of uncommitted.", view.State.Dirty)
	case view.State.Dirty > 0 || view.State.Unpushed > 0:
		return fmt.Sprintf("%d△ %d↑, nothing alarming.", view.State.Dirty, view.State.Unpushed)
	default:
		return "clean and pushed. rare."
	}
}
