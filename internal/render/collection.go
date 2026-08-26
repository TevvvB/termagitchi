package render

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/TevvvB/termagitchi/internal/config"
	"github.com/TevvvB/termagitchi/internal/den"
	"github.com/TevvvB/termagitchi/internal/identity"
	"github.com/TevvvB/termagitchi/internal/score"
)

// rarityHue keeps a band's colour consistent across the party, den and hatch.
// UpdateNotice is one dim line, shown only under views a person asked for.
func UpdateNotice(latest, command string) string {
	if latest == "" {
		return ""
	}
	return fmt.Sprintf("  %supdate available: %s · %s%s\n\n", dim, latest, command, reset)
}

func rarityHue(rarity identity.Rarity) string {
	switch rarity {
	case identity.Mythic:
		return "\033[38;5;213m"
	case identity.Legendary:
		return "\033[38;5;141m"
	case identity.Rare:
		return "\033[38;5;81m"
	case identity.Uncommon:
		return "\033[38;5;79m"
	default:
		return "\033[38;5;252m"
	}
}

// Party is the view no single-pet companion can render: every live worktree at
// once, with the worst signal across all of them called out at the bottom.
func Party(views []View, settings config.Config, showAll bool, now time.Time) string {
	if len(views) == 0 {
		return fmt.Sprintf("%s(-.-) no worktrees seen yet. open one.%s\n", dim, reset)
	}
	// Worst first: this is a health dashboard before it is a trophy case, and
	// sorting by branch below keeps the order stable between refreshes.
	sort.Slice(views, func(first, second int) bool {
		if views[first].Score.Hearts != views[second].Score.Hearts {
			return views[first].Score.Hearts < views[second].Score.Hearts
		}
		return views[first].Branch < views[second].Branch
	})

	// Sorted worst-first, so anything hidden is healthy by construction. Without
	// this a machine running two dozen worktrees prints a wall of identical
	// full-health rows and scrolls the one that needs attention off the top.
	alive := len(views)
	hidden := 0
	if limit := settings.Display.PartyLimit; !showAll && limit > 0 && alive > limit {
		hidden = alive - limit
		views = views[:limit]
	}

	living := 0
	for _, view := range views {
		for _, resident := range view.Residents {
			if !resident.Stale(now) {
				living++
			}
		}
	}
	var out strings.Builder
	// "alive" used to count worktrees, which is what made it a lie: a directory
	// existing says nothing about whether an agent is running in it.
	summary := fmt.Sprintf("%d dens", alive)
	if living > 0 {
		summary = fmt.Sprintf("%d dens · %d agent%s", alive, living,
			map[bool]string{true: "", false: "s"}[living == 1])
	}
	fmt.Fprintf(&out, "\n  %spets party%s%s%s%s%s\n\n",
		label, reset, strings.Repeat(" ", 30), dim, summary, reset)

	widestSpecies, widestBranch := 0, 0
	for _, view := range views {
		if width := displayWidth(view.Pet.Label()); width > widestSpecies {
			widestSpecies = width
		}
		if width := displayWidth(view.Branch); width > widestBranch {
			widestBranch = width
		}
	}
	if widestBranch > settings.Display.BranchLabelMax {
		widestBranch = settings.Display.BranchLabelMax
	}

	worst := views[0]
	for _, view := range views {
		body := hue(view.Pet.Color) + pad(view.Body(), 9) + reset
		species := pad(view.Pet.Label(), widestSpecies)
		branch := pad(truncate(view.Branch, settings.Display.BranchLabelMax), widestBranch)

		// Tighter than the status line's "@ XYZ": this is a column to scan down,
		// so it loses the space and keeps the marker.
		home := ""
		if code := view.Place.Code; code != "" {
			home = dim + pad("@"+code, 5) + reset
		}
		fmt.Fprintf(&out, "  %s%s %s%s%s %s%-5s%s %s%s%s  %s",
			home,
			body,
			hue(view.Pet.Color), species, reset,
			rarityHue(view.Pet.Rarity), view.Pet.Rarity.Stars(), reset,
			label, branch, reset,
			view.hearts())
		if flags := view.Flags(settings); len(flags) > 0 {
			fmt.Fprintf(&out, "  %s%s%s", warn, strings.Join(flags, " "), reset)
		}
		fmt.Fprintln(&out)
		// The agents living in this den, which is the thing a worktree-shaped
		// list could never show: two agents here used to be one row.
		// A den with one resident has already drawn that creature on its own row,
		// so repeating it reads as a rendering fault rather than as detail.
		alone := len(view.Residents) == 1
		for _, resident := range view.Residents {
			pet := identity.For(resident.Session)
			tone, note := hue(pet.Color), ""
			if resident.Stale(now) {
				tone, note = dim, dim+" · stale"+reset
			} else if resident.JustArrived(now) {
				note = dim + " · just arrived" + reset
			}
			creature, species := pad(pet.Prefix+view.face()+pet.Suffix, 9), pad(pet.Name, 8)
			if alone {
				creature, species = pad("", 9), pad("", 8)
			}
			fmt.Fprintf(&out, "       %s%s %s%s  %s%s%s%s%s\n",
				tone, creature, species, reset,
				dim, pad(truncate(resident.Label(), 36), 38), since(resident.Seen, now), note,
				contextNote(resident.Context))
		}
	}

	if hidden > 0 {
		healthiest := views[len(views)-1].Score.Hearts
		note := fmt.Sprintf("and %d more", hidden)
		if healthiest >= score.Max {
			note = fmt.Sprintf("and %d more, all at full health", hidden)
		}
		fmt.Fprintf(&out, "  %s%s · pets party --all%s\n", dim, note, reset)
	}

	if len(worst.Score.Penalties) > 0 {
		// A signal can charge twice, once per threshold, but naming it twice
		// reads as a bug rather than as emphasis.
		seen := map[string]bool{}
		reasons := make([]string, 0, len(worst.Score.Penalties))
		for _, penalty := range worst.Score.Penalties {
			if seen[penalty.Signal] {
				continue
			}
			seen[penalty.Signal] = true
			reasons = append(reasons, penalty.Signal)
		}
		fmt.Fprintf(&out, "\n  %sworst:%s %s%s%s %s·%s %s%s%s\n",
			dim, reset, hue(worst.Pet.Color), worst.Pet.Name, reset,
			dim, reset, warn, strings.Join(reasons, ", "), reset)
	}
	fmt.Fprintln(&out)
	return out.String()
}

// Places is the other half of the collection: which cities have been worked in.
//
// This is what identity.ArtWidth exists for. Every drawing is the same width, so
// a grid needs no measuring, and the codes can be centred underneath by
// arithmetic rather than by rendering and inspecting.
func Places(visited []identity.Place) string {
	if len(visited) == 0 {
		return ""
	}
	const perRow = 4
	const gap = 4
	var out strings.Builder
	fmt.Fprintf(&out, "\n  %splaces worked in%s\n\n", dim, reset)
	for start := 0; start < len(visited); start += perRow {
		end := start + perRow
		if end > len(visited) {
			end = len(visited)
		}
		row := visited[start:end]
		for line := 0; line < identity.ArtRows; line++ {
			out.WriteString("  ")
			for column, place := range row {
				if column > 0 {
					out.WriteString(strings.Repeat(" ", gap))
				}
				drawing := place.Art[line]
				if column == len(row)-1 {
					// The drawings are padded to ArtWidth, which is what makes the
					// grid line up, but on the last column that padding is just
					// trailing whitespace on the line.
					drawing = strings.TrimRight(drawing, " ")
				}
				fmt.Fprintf(&out, "%s%s%s", dim, drawing, reset)
			}
			out.WriteString("\n")
		}
		out.WriteString("  ")
		for column, place := range row {
			if column > 0 {
				out.WriteString(strings.Repeat(" ", gap))
			}
			// Centred by arithmetic, which only works because every drawing is
			// exactly ArtWidth wide. The right pad is dropped on the last column
			// so no line carries trailing whitespace.
			left := (identity.ArtWidth - len(place.Code)) / 2
			fmt.Fprintf(&out, "%s%s%s%s", strings.Repeat(" ", left), label, place.Code, reset)
			if column < len(row)-1 {
				fmt.Fprint(&out, strings.Repeat(" ", identity.ArtWidth-len(place.Code)-left))
			}
		}
		out.WriteString("\n\n")
	}
	return out.String()
}

// Den is the collection view: what you have hatched, and what is still missing.
func Den(collection den.Den, visited []identity.Place) string {
	var out strings.Builder
	progress := collection.Completion()
	have, total := 0, 0
	for _, band := range progress {
		have += band.Have
		total += band.Total
	}

	fmt.Fprintf(&out, "\n  %spets den%s%s%d/%d species · %d shiny · %d/%d places%s\n\n",
		label, reset, strings.Repeat(" ", 14), have, total, collection.ShinyCount(),
		len(visited), len(identity.AllPlaces()), reset)

	const width = 20
	for _, band := range progress {
		filled := 0
		if band.Total > 0 {
			filled = band.Have * width / band.Total
		}
		fmt.Fprintf(&out, "  %s%-11s%s %s%s%s%s%s  %s%d/%d%s\n",
			rarityHue(band.Rarity), strings.ToUpper(band.Rarity.String()), reset,
			rarityHue(band.Rarity), strings.Repeat("█", filled),
			dim, strings.Repeat("░", width-filled), reset,
			dim, band.Have, band.Total, reset)
	}

	out.WriteString(Places(visited))

	if latest := collection.Latest(1); len(latest) > 0 {
		entry := latest[0]
		fmt.Fprintf(&out, "\n  %slatest%s  %s%s%s  %s%s%s  %s%s · %s%s\n",
			dim, reset,
			label, entry.Species, reset,
			rarityHue(rarityByName(entry.Rarity)), entry.Rarity, reset,
			dim, entry.Branch, humanAge(entry.FirstSeen), reset)
	}
	fmt.Fprintln(&out)
	return out.String()
}

// Hatch is the one animated moment in the project: a branch never opened before.
//
// collected is false when the worktree has no commit yet, in which case the
// creature is shown but has not entered the den.
func Hatch(pet identity.Pet, have, total int, collected bool) string {
	var out strings.Builder
	tint := rarityHue(pet.Rarity)
	fmt.Fprintf(&out, "\n       %s.-\"\"\"-.%s\n", dim, reset)
	fmt.Fprintf(&out, "      %s/  . .  \\%s       %sa branch you have not opened before%s\n", dim, reset, dim, reset)
	fmt.Fprintf(&out, "     %s|  .   . |%s\n", dim, reset)
	fmt.Fprintf(&out, "      %s\\  ...  /%s\n", dim, reset)
	fmt.Fprintf(&out, "       %s'-...-'%s\n\n", dim, reset)
	fmt.Fprintf(&out, "       %s%s%s\n\n", hue(pet.Color), pet.Prefix+score.Face(score.Max)+pet.Suffix, reset)
	fmt.Fprintf(&out, "       %s%s%s  %s%s %s%s\n",
		hue(pet.Color), pet.Label(), reset,
		tint, pet.Rarity.Stars(), strings.ToUpper(pet.Rarity.String()), reset)
	if collected {
		fmt.Fprintf(&out, "       %sfirst hatch · %d of %d in your den%s\n\n", dim, have, total, reset)
	} else {
		fmt.Fprintf(&out, "       %sfirst hatch · commit here to keep it%s\n\n", dim, reset)
	}
	return out.String()
}

func rarityByName(name string) identity.Rarity {
	for band := identity.Common; band <= identity.Mythic; band++ {
		if band.String() == name {
			return band
		}
	}
	return identity.Common
}

func humanAge(when time.Time) string {
	elapsed := time.Since(when)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(elapsed.Hours()/24))
	}
}
