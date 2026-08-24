package render

import (
	"fmt"
	"strings"
	"testing"

	"github.com/TevvvB/parallel-harness-pets/internal/agents"
	"github.com/TevvvB/parallel-harness-pets/internal/config"
	"github.com/TevvvB/parallel-harness-pets/internal/identity"
	"github.com/TevvvB/parallel-harness-pets/internal/score"
	"github.com/TevvvB/parallel-harness-pets/internal/state"
	"time"
)

// A terminal renders an emoji in two cells but Go counts it as one rune, so
// padding from a rune count misaligns every column after a shiny's sparkle.
func TestDisplayWidthCountsTerminalCells(t *testing.T) {
	cases := []struct {
		text  string
		width int
	}{
		{"gecko", 5},
		{"gecko ✨", 8},
		{"", 0},
		{">(•ᴗ•)<", 7},
		{"~(•ᴗ•)~", 7},
		{"✦✦✦", 3},
		{"♥♥♥♡♡", 5},
		{"△↑↓⑂✗", 5},
		{"████░░", 6},
		{"feat/機能追加", 5 + 8},
	}
	for _, testCase := range cases {
		if got := displayWidth(testCase.text); got != testCase.width {
			t.Errorf("displayWidth(%q) = %d, want %d", testCase.text, got, testCase.width)
		}
	}
}

func TestPadFillsToCellWidth(t *testing.T) {
	if got := displayWidth(pad("gecko ✨", 10)); got != 10 {
		t.Errorf("pad to 10 produced %d cells", got)
	}
	if got := displayWidth(pad("ant", 10)); got != 10 {
		t.Errorf("pad to 10 produced %d cells", got)
	}
	// Padding must never truncate.
	if !strings.HasPrefix(pad("a-very-long-branch-name", 3), "a-very-long-branch-name") {
		t.Error("pad truncated instead of leaving the text alone")
	}
}

func view(branch string, hearts int, shiny bool) View {
	pet := identity.For(branch)
	pet.Shiny = shiny
	return View{
		Pet: pet, Branch: branch, HasState: true,
		Score: score.Result{Hearts: hearts},
		State: state.State{External: map[string]int{}},
	}
}

// Every party row must start its columns at the same cell, shiny or not.
func TestPartyColumnsAlign(t *testing.T) {
	views := []View{
		view("feat/oauth-flow", 4, true),
		view("chore/deps-bump", 4, false),
		view("spike/graphql", 1, false),
	}
	rendered := stripANSI(Party(views, config.Default(), false, time.Now()))

	var starts []int
	for _, line := range strings.Split(rendered, "\n") {
		index := strings.Index(line, "✦")
		if index < 0 || strings.Contains(line, "pets party") {
			continue
		}
		starts = append(starts, displayWidth(line[:index]))
	}
	if len(starts) < 3 {
		t.Fatalf("expected 3 creature rows, found %d", len(starts))
	}
	for _, start := range starts[1:] {
		if start != starts[0] {
			t.Errorf("rarity column starts at %v across rows, want them equal", starts)
			break
		}
	}
}

// Worst first, because this is a health dashboard before it is a trophy case.
func TestPartySortsWorstFirst(t *testing.T) {
	views := []View{
		view("chore/deps-bump", 5, false),
		view("spike/graphql", 1, false),
		view("feat/oauth-flow", 3, false),
	}
	rendered := stripANSI(Party(views, config.Default(), false, time.Now()))
	positions := []int{
		strings.Index(rendered, "spike/graphql"),
		strings.Index(rendered, "feat/oauth-flow"),
		strings.Index(rendered, "chore/deps-bump"),
	}
	for index := 1; index < len(positions); index++ {
		if positions[index] < positions[index-1] {
			t.Errorf("party is not sorted worst-first: %v", positions)
			break
		}
	}
}

func TestPartyWithNoWorktrees(t *testing.T) {
	if out := Party(nil, config.Default(), false, time.Now()); !strings.Contains(out, "no worktrees") {
		t.Errorf("empty party rendered %q", out)
	}
}

// The renderer must never emit a bare newline-free blob or leave colour on.
func TestStatuslineResetsColour(t *testing.T) {
	rendered := Statusline(view("feat/oauth-flow", 4, false), config.Default())
	if !strings.HasSuffix(rendered, reset) {
		t.Error("status line does not end with a colour reset, so it would bleed into the bar")
	}
	if strings.Contains(rendered, "\n") {
		t.Error("status line contains a newline")
	}
}

func stripANSI(text string) string {
	var out strings.Builder
	inEscape := false
	for _, char := range text {
		switch {
		case char == 0x1b:
			inEscape = true
		case inEscape && char == 'm':
			inEscape = false
		case !inEscape:
			out.WriteRune(char)
		}
	}
	return out.String()
}

// A machine running two dozen worktrees would otherwise print a wall of
// identical full-health rows and scroll the one that needs attention off the top.
func TestPartyCapsTheListAndSaysWhatItHid(t *testing.T) {
	var views []View
	views = append(views, view("spike/graphql", 1, false))
	for index := 0; index < 20; index++ {
		views = append(views, view(fmt.Sprintf("agent-task-%d", index), 5, false))
	}

	settings := config.Default()
	rendered := stripANSI(Party(views, settings, false, time.Now()))

	rows := creatureRows(rendered)
	if rows != settings.Display.PartyLimit {
		t.Errorf("rendered %d creature rows, want the limit of %d", rows, settings.Display.PartyLimit)
	}
	if !strings.Contains(rendered, "and 9 more, all at full health") {
		t.Errorf("did not report what it hid:\n%s", rendered)
	}
	// The count must stay honest even though the list is truncated. It counts
	// dens rather than "alive", because a worktree existing never meant an agent
	// was running in it.
	if !strings.Contains(rendered, "21 dens") {
		t.Error("den count does not reflect every worktree")
	}
	// The one that matters must survive the cut.
	if !strings.Contains(rendered, "spike/graphql") {
		t.Error("the worst worktree was truncated away")
	}
}

func TestPartyAllShowsEverything(t *testing.T) {
	var views []View
	for index := 0; index < 20; index++ {
		views = append(views, view(fmt.Sprintf("agent-task-%d", index), 5, false))
	}
	rendered := stripANSI(Party(views, config.Default(), true, time.Now()))
	if got := creatureRows(rendered); got != 20 {
		t.Errorf("--all showed %d worktrees, want 20", got)
	}
	if strings.Contains(rendered, "more") {
		t.Error("--all still reported hidden rows")
	}
}

// creatureRows counts rendered worktrees. Rarity stars cannot be counted
// directly, because one row carries between one and five of them.
func creatureRows(rendered string) int {
	rows := 0
	for _, line := range strings.Split(rendered, "\n") {
		if strings.ContainsAny(line, "♥♡") {
			rows++
		}
	}
	return rows
}

// The places grid centres codes by arithmetic and wraps at a fixed width, both
// of which are easy to get subtly wrong and invisible until someone squints at
// a terminal. It also must not emit trailing whitespace.
func TestPlacesGridAlignsAndWraps(t *testing.T) {
	roster := identity.AllPlaces()
	if len(roster) < 6 {
		t.Skip("roster too small to exercise wrapping")
	}
	rendered := stripANSI(Places(roster[:6]))
	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")

	for i, line := range lines {
		if strings.HasSuffix(line, " ") {
			t.Errorf("line %d has trailing whitespace: %q", i, line)
		}
	}
	// Six places at four per row means two rows, and each row is three drawing
	// lines plus a code line.
	codeLines := 0
	for _, line := range lines {
		if strings.Contains(line, roster[0].Code) || strings.Contains(line, roster[4].Code) {
			codeLines++
		}
	}
	if codeLines != 2 {
		t.Errorf("expected 2 code rows for 6 places at 4 per row, found %d", codeLines)
	}
	// A code must sit under its own drawing, not at the left margin.
	for _, line := range lines {
		if strings.Contains(line, roster[0].Code) {
			if strings.HasPrefix(strings.TrimLeft(line, " "), roster[0].Code) &&
				len(line)-len(strings.TrimLeft(line, " ")) < 3 {
				t.Errorf("code not centred under its drawing: %q", line)
			}
		}
	}
	if Places(nil) != "" {
		t.Error("an empty roster still rendered a grid")
	}
}

// A den with one resident drew its creature on both lines, which reads as a
// rendering fault. Confirmed in real use across four dens on one screen.
func TestPartyDoesNotRepeatTheCreatureOfALoneResident(t *testing.T) {
	now := time.Now()
	den := view("feat/solo", 5, false)
	den.Den = "place:solo#solo"
	den.Residents = []agents.Record{{Session: "solo-session", Den: den.Den, Seen: now, Since: now}}

	rendered := stripANSI(Party([]View{den}, config.Default(), false, now))
	body := identity.For("solo-session").Prefix
	if count := strings.Count(rendered, body); count != 1 {
		t.Errorf("resident's creature drawn %d times, want once\n%s", count, rendered)
	}

	// Two residents still need their own creatures, or the rows stop being
	// distinguishable, which is the whole point of listing them.
	den.Residents = append(den.Residents,
		agents.Record{Session: "second-session", Den: den.Den, Seen: now, Since: now})
	rendered = stripANSI(Party([]View{den}, config.Default(), false, now))
	for _, session := range []string{"solo-session", "second-session"} {
		if !strings.Contains(rendered, identity.For(session).Name) {
			t.Errorf("resident %q lost its species in a shared den\n%s", session, rendered)
		}
	}
}

// The Codex Stop hook gets one line and no status bar, and it used to carry only the
// creature and a sentence: the score and two of the three counts were simply absent, so
// the readout barely registered among the host's own output.
func TestHookMessageCarriesScoreAndCounts(t *testing.T) {
	view := View{
		Pet:      identity.For("main"),
		Place:    identity.PlaceForKey("place:demo#demo"),
		HasState: true,
		Score:    score.Result{Hearts: 1},
		State:    state.State{Dirty: 31, Unpushed: 27, Behind: 250},
	}
	message := HookMessage(view, config.Default(), "this branch only exists on your laptop.")

	// Three rows: Codex puts the first after its own prefix and indents the rest, so the
	// split is what makes this read as a block rather than one trailing sentence.
	rows := strings.Split(message, "\n")
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d: %q", len(rows), message)
	}
	if !strings.Contains(rows[0], view.Pet.Name) || !strings.Contains(rows[0], "@ "+view.Place.Code) {
		t.Errorf("first row should say who and where, got %q", rows[0])
	}
	for _, want := range []string{"♥♡♡♡♡", "31△", "27↑", "250↓"} {
		if !strings.Contains(rows[1], want) {
			t.Errorf("state row %q is missing %q", rows[1], want)
		}
	}
	// A host that prints this as body text may not survive raw escapes, and we cannot
	// know its palette either way.
	if strings.Contains(message, "\033") {
		t.Errorf("hook message carries ANSI: %q", message)
	}
}
