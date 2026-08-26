// Package render turns a resolved pet into text for whichever surface asked.
//
// One code path feeds the Claude Code status line, a tmux segment, a terminal
// title, and JSON for editor extensions.
package render

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/TevvvB/termagitchi/internal/agents"
	"github.com/TevvvB/termagitchi/internal/config"
	"github.com/TevvvB/termagitchi/internal/identity"
	"github.com/TevvvB/termagitchi/internal/score"
	"github.com/TevvvB/termagitchi/internal/state"
	"github.com/TevvvB/termagitchi/internal/visits"
)

const (
	reset    = "\033[0m"
	dim      = "\033[38;5;240m"
	label    = "\033[38;5;252m"
	warn     = "\033[38;5;179m"
	heartHue = "\033[38;5;210m"
)

// View is everything a surface needs, already resolved.
type View struct {
	Pet   identity.Pet
	Place identity.Place
	// Den is the storage key for this worktree, and Session identifies which
	// agent is asking, so a surface can mark "you" among a den's residents.
	Den       string
	Session   string
	Residents []agents.Record
	// Visitors is everyone who has ever worked in this den, including the ones
	// who have long since moved on.
	Visitors []visits.Visit
	Branch   string
	Root     string
	Score    score.Result
	State    state.State
	Tests    string
	Model    string
	HasState bool
}

// Home is the den's compact form: the flight code, marked with an "at" so a
// reader who has never seen this tool still parses three capitals as a place
// rather than as an abbreviation they are expected to know.
func (v View) Home() string {
	if v.Place.Code == "" {
		return ""
	}
	return "@ " + v.Place.Code
}

func hue(color int) string {
	return fmt.Sprintf("\033[38;5;%dm", color)
}

func (v View) face() string {
	if !v.HasState {
		return "-.-"
	}
	return score.Face(v.Score.Hearts)
}

// Body is the creature itself, frame and all.
func (v View) Body() string {
	return v.Pet.Prefix + v.face() + v.Pet.Suffix
}

func (v View) hearts() string {
	filled, empty := v.heartGlyphs()
	if !v.HasState {
		return dim + empty + reset
	}
	return heartHue + filled + dim + empty + reset
}

// heartGlyphs splits the bar so a surface that cannot take ANSI still shows the score.
func (v View) heartGlyphs() (filled, empty string) {
	if !v.HasState {
		return "", "·····"
	}
	return strings.Repeat("♥", v.Score.Hearts), strings.Repeat("♡", score.Max-v.Score.Hearts)
}

// HookMessage is the whole readout for a surface that gets one line and no status bar.
//
// Codex splits systemMessage on newlines, renders the first row inline after its own
// "says:" prefix, and indents every later row by four spaces, uncapped: only its separate
// hook-context path truncates at three. The first row is left empty so the creature,
// the state and the voice all land in the indented block and line up with each other.
//
// Deliberately monochrome: the host prints this as body text, raw ANSI is not guaranteed
// to survive, and its palette is unknowable from here.
func HookMessage(v View, settings config.Config, quip string) string {
	who := []string{v.Body(), v.Pet.Name}
	if home := v.Home(); home != "" {
		who = append(who, home)
	}
	// Leading blank so the creature gets its own row: Codex renders the first row inline
	// after its "says:" prefix, and only later rows land on the indented block.
	rows := []string{"", strings.Join(who, " ")}
	if state := v.stateRow(settings); state != "" {
		rows = append(rows, state)
	}
	return strings.Join(append(rows, quip), "\n")
}

// stateRow is the score and counts a status line would show, without colour.
func (v View) stateRow(settings config.Config) string {
	if !v.HasState {
		return ""
	}
	filled, empty := v.heartGlyphs()
	row := filled + empty
	if flags := v.Flags(settings); len(flags) > 0 {
		row += "  " + strings.Join(flags, " ")
	}
	return row
}

// Flags are the raw counts behind the mood, shown only when they are non-zero.
func (v View) Flags(settings config.Config) []string {
	var flags []string
	if v.State.Dirty > 0 {
		flags = append(flags, fmt.Sprintf("%d△", v.State.Dirty))
	}
	if v.State.Unpushed > 0 {
		flags = append(flags, fmt.Sprintf("%d↑", v.State.Unpushed))
	}
	if v.State.Behind > settings.Display.BehindShownPast {
		flags = append(flags, fmt.Sprintf("%d↓", v.State.Behind))
	}
	if v.State.Migrations > 1 {
		flags = append(flags, fmt.Sprintf("⑂%d", v.State.Migrations))
	}
	if v.Tests == "fail" {
		flags = append(flags, "✗")
	}
	return flags
}

// wideRanges are the codepoints a terminal draws in two cells: the East Asian
// Wide blocks, and the emoji that have emoji presentation by default.
//
// The Miscellaneous Symbols block cannot be treated as a whole, because ✨ is
// wide while ✦ and ♥ sit beside it and are narrow.
var wideRanges = [][2]rune{
	{0x1100, 0x115F}, {0x231A, 0x231B}, {0x23E9, 0x23EC}, {0x23F0, 0x23F0},
	{0x23F3, 0x23F3}, {0x25FD, 0x25FE}, {0x2614, 0x2615}, {0x2648, 0x2653},
	{0x267F, 0x267F}, {0x2693, 0x2693}, {0x26A1, 0x26A1}, {0x26AA, 0x26AB},
	{0x26BD, 0x26BE}, {0x26C4, 0x26C5}, {0x26CE, 0x26CE}, {0x26D4, 0x26D4},
	{0x26EA, 0x26EA}, {0x26F2, 0x26F3}, {0x26F5, 0x26F5}, {0x26FA, 0x26FA},
	{0x26FD, 0x26FD}, {0x2705, 0x2705}, {0x270A, 0x270B}, {0x2728, 0x2728},
	{0x274C, 0x274C}, {0x274E, 0x274E}, {0x2753, 0x2755}, {0x2757, 0x2757},
	{0x2795, 0x2797}, {0x27B0, 0x27B0}, {0x27BF, 0x27BF}, {0x2B1B, 0x2B1C},
	{0x2B50, 0x2B50}, {0x2B55, 0x2B55}, {0x2E80, 0x303E}, {0x3041, 0x33FF},
	{0x3400, 0x4DBF}, {0x4E00, 0x9FFF}, {0xA000, 0xA4CF}, {0xAC00, 0xD7A3},
	{0xF900, 0xFAFF}, {0xFE30, 0xFE6F}, {0xFF00, 0xFF60}, {0xFFE0, 0xFFE6},
	{0x1F300, 0x1FAFF},
}

// displayWidth counts terminal cells, not runes.
//
// An emoji is one rune but occupies two cells, so padding computed from a rune
// count misaligns every column after a shiny creature's sparkle. The same
// applies to a branch name written in a wide script.
func displayWidth(text string) int {
	width := 0
	for _, char := range text {
		width++
		for _, span := range wideRanges {
			if char >= span[0] && char <= span[1] {
				width++
				break
			}
		}
	}
	return width
}

// pad right-fills to a target number of terminal cells.
func pad(text string, target int) string {
	if gap := target - displayWidth(text); gap > 0 {
		return text + strings.Repeat(" ", gap)
	}
	return text
}

func truncate(text string, max int) string {
	if max <= 1 || len([]rune(text)) <= max {
		return text
	}
	return string([]rune(text)[:max-1]) + "…"
}

// Statusline is the one-line form Claude Code refreshes every second.
func Statusline(v View, settings config.Config) string {
	color := hue(v.Pet.Color)
	// The pet leads because it is the subject; the den trails it as context and
	// stays dim so it never competes with the creature for attention.
	parts := []string{
		color + v.Body() + reset,
		color + v.Pet.Name + reset,
	}
	if home := v.Home(); home != "" {
		parts = append(parts, dim+home+reset)
	}
	parts = append(parts,
		dim+"·"+reset,
		label+truncate(v.Branch, settings.Display.BranchLabelMax)+reset,
		v.hearts(),
	)
	if flags := v.Flags(settings); v.HasState && len(flags) > 0 {
		parts = append(parts, warn+strings.Join(flags, " ")+reset)
	}
	if v.Model != "" {
		parts = append(parts, dim+"·"+reset, dim+v.Model+reset)
	}
	return strings.Join(parts, " ")
}

// Tmux uses tmux's own colour syntax so the segment inherits the bar's styling.
func Tmux(v View, settings config.Config) string {
	out := fmt.Sprintf("#[fg=colour%d]%s %s#[default] %s",
		v.Pet.Color, v.Body(), v.Pet.Name, truncate(v.Branch, settings.Display.BranchLabelMax))
	if flags := v.Flags(settings); v.HasState && len(flags) > 0 {
		out += " #[fg=colour179]" + strings.Join(flags, " ") + "#[default]"
	}
	return out
}

// Title is plain text for a terminal or tab title, where escapes cannot go.
func Title(v View, settings config.Config) string {
	out := fmt.Sprintf("%s %s · %s", v.Body(), v.Pet.Name,
		truncate(v.Branch, settings.Display.BranchLabelMax))
	if flags := v.Flags(settings); v.HasState && len(flags) > 0 {
		out += " " + strings.Join(flags, " ")
	}
	return out
}

// JSON is the contract for editor extensions and any surface not yet invented.
func JSON(v View, settings config.Config) (string, error) {
	payload := map[string]any{
		"species":  v.Pet.Name,
		"body":     v.Body(),
		"color":    v.Pet.Color,
		"branch":   v.Branch,
		"root":     v.Root,
		"hearts":   v.Score.Hearts,
		"max":      score.Max,
		"flags":    v.Flags(settings),
		"tests":    v.Tests,
		"ready":    v.HasState,
		"dirty":    v.State.Dirty,
		"unpushed": v.State.Unpushed,
		"behind":   v.State.Behind,
	}
	if len(v.State.External) > 0 {
		payload["external"] = v.State.External
	}
	encoded, err := json.Marshal(payload)
	return string(encoded), err
}

// since is the human form of how long ago an agent was last heard from, which
// is the only evidence available that it is still running.
func since(then, now time.Time) string {
	gap := now.Sub(then)
	switch {
	case gap < 45*time.Second:
		return "just now"
	case gap < time.Hour:
		return fmt.Sprintf("%dm ago", int(gap.Minutes()))
	case gap < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(gap.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(gap.Hours())/24)
	}
}

// Residents lists the agents sharing this den, marking which one is asking.
//
// This is the view the old model could not express: a worktree held one
// creature, so a second agent in the same worktree was invisible.
// ContextFull is where the figure stops being dim and starts warning: an agent near the end of its window is actionable.
const ContextFull = 80

// contextNote renders the fill, or nothing when unknown, so a hook-only harness is never shown a confident 0%.
func contextNote(percent int) string {
	if percent <= 0 {
		return ""
	}
	tone := dim
	if percent >= ContextFull {
		tone = warn
	}
	return fmt.Sprintf(" %s· %d%%%s", tone, percent, reset)
}

func Residents(v View, now time.Time) string {
	if len(v.Residents) == 0 {
		return ""
	}
	var out strings.Builder
	living := 0
	for _, resident := range v.Residents {
		if !resident.Stale(now) {
			living++
		}
	}
	fmt.Fprintf(&out, "\n  %s%d agent%s in this den%s\n\n",
		dim, living, map[bool]string{true: "", false: "s"}[living == 1], reset)

	// Mood belongs to the worktree, so every agent in a den wears the same face.
	// A fixed cheerful one here would be a readout that reports nothing.
	face := v.face()
	width := 0
	for _, resident := range v.Residents {
		pet := identity.For(resident.Session)
		if size := displayWidth(pet.Prefix + face + pet.Suffix); size > width {
			width = size
		}
	}
	for _, resident := range v.Residents {
		pet := identity.For(resident.Session)
		body := pet.Prefix + face + pet.Suffix
		marker, tone := "  ", hue(pet.Color)
		if resident.Session == v.Session {
			marker = dim + "→ " + reset
		}
		// A stale agent is dimmed rather than hidden: an agent that crashed and
		// is still on screen is more use than one that silently disappeared.
		note := ""
		if resident.Stale(now) {
			tone, note = dim, dim+"  stale"+reset
		} else if resident.JustArrived(now) {
			note = dim + "  just arrived" + reset
		}
		fmt.Fprintf(&out, "  %s%s%s%s  %s%s%s  %s%s%s%s%s\n",
			marker, tone, pad(body, width), reset,
			tone, pad(pet.Name, 9), reset,
			dim, pad(truncate(resident.Label(), 32), 34), since(resident.Seen, now), note,
			contextNote(resident.Context))
	}
	return out.String()
}

// PassedThrough is the den's memory: agents that worked here and have gone.
//
// Only the ones no longer present are listed, because the ones still here are
// already above under Residents and repeating them reads as double counting.
func PassedThrough(v View, now time.Time) string {
	here := map[string]bool{}
	for _, resident := range v.Residents {
		here[resident.Session] = true
	}
	var gone []visits.Visit
	for _, visit := range v.Visitors {
		if !here[visit.Session] {
			gone = append(gone, visit)
		}
	}
	if len(gone) == 0 {
		return ""
	}
	var out strings.Builder
	fmt.Fprintf(&out, "\n  %s%d passed through%s\n\n", dim, len(gone), reset)
	for _, visit := range gone {
		pet := identity.For(visit.Session)
		fmt.Fprintf(&out, "    %s%s  %s%s  %s%s\n",
			hue(pet.Color), pad(pet.Prefix+"-.-"+pet.Suffix, 9),
			pad(pet.Name, 9), reset,
			dim+"last here "+since(visit.Last, now), reset)
	}
	return out.String()
}

// Card is the full readout, with every penalised signal called out in amber.
func Card(v View, settings config.Config) string {
	var out strings.Builder
	color := hue(v.Pet.Color)

	penalised := map[string]bool{}
	for _, penalty := range v.Score.Penalties {
		penalised[penalty.Signal] = true
	}
	row := func(name, value string, flagged bool) {
		shown := value
		if flagged {
			shown = warn + value + reset
		}
		fmt.Fprintf(&out, "  %s%-14s%s %s\n", dim, name, reset, shown)
	}

	// The card has room, so the den gets its drawing here. The status line does
	// not, and only ever sees the glyph and the code.
	if v.Place.Code != "" {
		fmt.Fprintf(&out, "\n  %s%s  %s%s\n", dim, v.Home(), v.Place.Name, reset)
		for _, line := range v.Place.Art {
			fmt.Fprintf(&out, "  %s%s%s\n", dim, line, reset)
		}
	}
	fmt.Fprintf(&out, "\n  %s%s%s  %s%s%s\n", color, v.Body(), reset, color, v.Pet.Name, reset)
	fmt.Fprintf(&out, "  %s%s%s\n", dim, strings.Repeat("─", 30), reset)
	if v.Place.Code != "" {
		row("den", fmt.Sprintf("%s (%s)", v.Place.Name, v.Place.Code), false)
	}
	row("branch", v.Branch, false)
	row("worktree", v.Root, false)
	row("mood", fmt.Sprintf("%s  %d/%d", v.hearts(), v.Score.Hearts, score.Max), false)
	fmt.Fprintln(&out)
	row("uncommitted", fmt.Sprint(v.State.Dirty), penalised["uncommitted"])
	row("unpushed", fmt.Sprint(v.State.Unpushed), penalised["unpushed"])
	row("behind trunk", fmt.Sprint(v.State.Behind), false)
	if settings.Signals.Migrations.Enabled {
		row("migration heads", fmt.Sprint(v.State.Migrations), penalised["migrations"])
	}
	row("last tests", v.Tests, penalised["tests"])
	for name, value := range v.State.External {
		row(name, fmt.Sprint(value), penalised[name])
	}
	now := time.Now()
	out.WriteString(Residents(v, now))
	out.WriteString(PassedThrough(v, now))
	fmt.Fprintln(&out)
	return out.String()
}
