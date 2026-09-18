package main

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/TevvvB/termagitchi/internal/render"
	"github.com/TevvvB/termagitchi/internal/verdict"
)

// blockingReader never reaches EOF, which is what tmux can hand a command it
// runs through #(...). An unbounded read there hangs the status bar forever.
type blockingReader struct{ released chan struct{} }

func (b blockingReader) Read([]byte) (int, error) {
	<-b.released
	return 0, io.EOF
}

func TestParsePayloadGivesUpOnAReaderThatNeverCloses(t *testing.T) {
	reader := blockingReader{released: make(chan struct{})}
	defer close(reader.released)

	done := make(chan hookPayload, 1)
	go func() { done <- parsePayload(reader, 50*time.Millisecond) }()

	select {
	case payload := <-done:
		if payload.Cwd != "" {
			t.Errorf("expected an empty payload on timeout, got %+v", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parsePayload blocked on a reader that never closes")
	}
}

func TestParsePayloadReadsHarnessJSON(t *testing.T) {
	body := `{"cwd":"/tmp/x","workspace":{"current_dir":"/tmp/y"},
	          "model":{"display_name":"Opus 5"},
	          "tool_input":{"command":"pytest"},"tool_response":"3 failed"}`
	payload := parsePayload(strings.NewReader(body), time.Second)

	if payload.Workspace.CurrentDir != "/tmp/y" {
		t.Errorf("workspace dir = %q", payload.Workspace.CurrentDir)
	}
	if payload.Model.DisplayName != "Opus 5" {
		t.Errorf("model = %q", payload.Model.DisplayName)
	}
	if payload.ToolInput.Command != "pytest" {
		t.Errorf("command = %q", payload.ToolInput.Command)
	}
	if !strings.Contains(string(payload.ToolResponse), "3 failed") {
		t.Errorf("tool response = %q", payload.ToolResponse)
	}
}

func TestParsePayloadToleratesGarbage(t *testing.T) {
	for _, body := range []string{"", "not json at all", "[]", "null"} {
		payload := parsePayload(strings.NewReader(body), time.Second)
		if payload.Cwd != "" {
			t.Errorf("parsePayload(%q) invented a cwd: %q", body, payload.Cwd)
		}
	}
}

// workspace.current_dir wins over cwd, because a harness sets the first to the
// worktree and the second to wherever the process happened to start.
func TestDirectoryPrefersTheWorkspace(t *testing.T) {
	real := t.TempDir()
	payload := hookPayload{Cwd: "/definitely/not/here"}
	payload.Workspace.CurrentDir = real
	if got := payload.directory(); got != real {
		t.Errorf("directory() = %q, want %q", got, real)
	}
}

func TestDirectoryFallsBackWhenPathsAreBogus(t *testing.T) {
	payload := hookPayload{Cwd: "/definitely/not/here"}
	if got := payload.directory(); got == "/definitely/not/here" {
		t.Error("directory() returned a path that does not exist")
	}
}

func TestFlagValue(t *testing.T) {
	args := []string{"--format=tmux", "--cwd=/tmp/x"}
	if got := flagValue(args, "format", "statusline"); got != "tmux" {
		t.Errorf("format = %q", got)
	}
	if got := flagValue(args, "cwd", ""); got != "/tmp/x" {
		t.Errorf("cwd = %q", got)
	}
	if got := flagValue(args, "missing", "fallback"); got != "fallback" {
		t.Errorf("missing flag = %q, want the fallback", got)
	}
}

// A raw cast yields JSON source, where a newline is still a backslash and an n.
// Substring checks survive that; anything anchored to a line never matches.
func TestResponseTextDecodesWhatTheRunnerPrinted(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"bare string", `"ok  \tgithub.com/x/y\t0.1s\n"`, "ok  \tgithub.com/x/y\t0.1s\n"},
		{"object with stdout", `{"stdout":"PASS\n","stderr":"","interrupted":false}`, "PASS\n"},
		{"object with both streams", `{"stdout":"ok\n","stderr":"warning\n"}`, "ok\n\nwarning\n"},
		{"empty", ``, ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := responseText(json.RawMessage(testCase.raw)); got != testCase.want {
				t.Errorf("responseText(%s) = %q, want %q", testCase.raw, got, testCase.want)
			}
		})
	}
}

// The end-to-end shape a harness actually sends: a green Go run has to clear a
// previous failure, which it cannot do while the text is still JSON-escaped.
func TestGreenGoRunIsRecognisedThroughAHarnessPayload(t *testing.T) {
	raw := json.RawMessage(`{"stdout":"ok  \tgithub.com/x/y\t0.101s\nok  \tgithub.com/x/z\t(cached)\n","stderr":""}`)
	result, ok := verdict.Of("go test ./...", responseText(raw))
	if !ok || result != "pass" {
		t.Errorf("green go run through a harness payload = %q/%v, want pass/true", result, ok)
	}
}

// Asking for help is a success. Bare pets already exited 0, so an exit 1 on
// --help was invisible interactively and broke every script and smoke test
// that ran it. A packaging check caught it, not a human.
func TestHelpExitsZero(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}, {}} {
		if code := dispatch(args); code != 0 {
			t.Errorf("dispatch(%v) = %d, want 0", args, code)
		}
	}
}

func TestUnknownCommandExitsNonZero(t *testing.T) {
	if code := dispatch([]string{"nonsense"}); code == 0 {
		t.Error("an unknown command exited 0, so a typo would look like success")
	}
}

func TestVersionExitsZero(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}, {"-v"}} {
		if code := dispatch(args); code != 0 {
			t.Errorf("dispatch(%v) = %d, want 0", args, code)
		}
	}
}

// A `go install pkg@vX.Y.Z` build carries no ldflag, so without a fallback it calls
// itself dev, and release.Newer deliberately ignores an unparseable version — meaning
// source installs never hear about an upgrade.
func TestModuleVersionNormalisesWhatTheGoToolRecords(t *testing.T) {
	for recorded, want := range map[string]string{
		"v0.2.9":  "0.2.9", // go install of a tag
		"(devel)": "dev",   // built from a working tree
		"":        "dev",   // no build info at all
	} {
		if got := moduleVersion(recorded); got != want {
			t.Errorf("moduleVersion(%q) = %q, want %q", recorded, got, want)
		}
	}
}

func TestParsePayloadReadsRateLimits(t *testing.T) {
	body := `{"cwd":"/tmp/x","rate_limits":{"five_hour":{"used_percentage":38,"resets_at":1786734600},"seven_day":{"used_percentage":10,"resets_at":1787248800}}}`
	payload := parsePayload(strings.NewReader(body), time.Second)
	if payload.RateLimits.FiveHour.UsedPercentage != 38 {
		t.Errorf("5h percent = %v", payload.RateLimits.FiveHour.UsedPercentage)
	}
	if payload.RateLimits.FiveHour.ResetsAt != 1786734600 {
		t.Errorf("5h resets_at = %d", payload.RateLimits.FiveHour.ResetsAt)
	}
	if payload.RateLimits.SevenDay.UsedPercentage != 10 {
		t.Errorf("7d percent = %v", payload.RateLimits.SevenDay.UsedPercentage)
	}
	who := payload.agent()
	if who.Rate5h != 38 || who.Rate7d != 10 || who.Rate5hReset != 1786734600 {
		t.Errorf("agent rate limits = %+v", who)
	}
}

func TestQuipForCompactsOnNearETA(t *testing.T) {
	want := "context is packing. /compact before it eats the thread."

	// ETA ≤15m with fill still under 80% should nudge /compact.
	if got := quipFor(render.View{Context: 50, ContextETA: 10 * time.Minute}); got != want {
		t.Errorf("ETA≤15m fill<80%% = %q, want compact quip", got)
	}
	// Exact threshold counts.
	if got := quipFor(render.View{Context: 50, ContextETA: render.ContextETACompact}); got != want {
		t.Errorf("ETA==15m fill<80%% = %q, want compact quip", got)
	}
	// Longer ETA and fill under 80% stays off the compact tier (clean default).
	if got := quipFor(render.View{Context: 50, ContextETA: 20 * time.Minute}); got == want {
		t.Errorf("ETA>15m fill<80%% still compact: %q", got)
	}
	// Fill at/above ContextFull still compact even with no ETA.
	if got := quipFor(render.View{Context: render.ContextFull}); got != want {
		t.Errorf("Context≥80%% = %q, want compact quip", got)
	}
}
func TestParsePayloadAcceptsFloatUsedPercentage(t *testing.T) {
	// Claude Code statusline docs send used_percentage as a float (e.g. 23.5).
	body := `{"cwd":"/tmp/x","context_window":{"used_percentage":72.4},"rate_limits":{"five_hour":{"used_percentage":38.6,"resets_at":1786734600},"seven_day":{"used_percentage":9.4,"resets_at":1787248800}},"model":{"display_name":"Opus"}}`
	payload := parsePayload(strings.NewReader(body), time.Second)
	who := payload.agent()
	if who.Context != 72 {
		t.Errorf("context whole percent = %d, want 72", who.Context)
	}
	if who.Rate5h != 39 {
		t.Errorf("5h whole percent = %d, want 39", who.Rate5h)
	}
	if who.Rate7d != 9 {
		t.Errorf("7d whole percent = %d, want 9", who.Rate7d)
	}
	if who.Model != "Opus" {
		t.Errorf("model = %q", who.Model)
	}
}

func TestQuipForWarnsOnRateLimitFull(t *testing.T) {
	want := "rate limit is packing. ease up before the window resets."
	if got := quipFor(render.View{Rate5h: render.ContextFull}); got != want {
		t.Errorf("5h≥80%% = %q, want rate-limit quip", got)
	}
	if got := quipFor(render.View{Rate7d: render.ContextFull}); got != want {
		t.Errorf("7d≥80%% = %q, want rate-limit quip", got)
	}
	// Context packing stays above the rate-limit tier.
	compact := "context is packing. /compact before it eats the thread."
	if got := quipFor(render.View{Context: render.ContextFull, Rate5h: render.ContextFull}); got != compact {
		t.Errorf("context+rate full = %q, want context compact quip", got)
	}
	// Under 80% stays off this tier.
	if got := quipFor(render.View{Rate5h: 79}); got == want {
		t.Errorf("5h 79%% still rate-limit quip: %q", got)
	}
}

