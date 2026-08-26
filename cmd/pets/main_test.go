package main

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

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
