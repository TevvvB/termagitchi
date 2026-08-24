// Package identity derives a creature from one opaque key.
//
// Nothing here touches disk or network: the same key yields the same creature on
// every machine, with nothing persisted. That holds for as long as the roster
// does - a creature is drawn with pool[hash%len], so growing a band reassigns
// every key already in it, which CONTRIBUTING.md covers as a release-sized event. Rarity is a weighted band over
// the same hash, so a collection costs no extra state.
//
// Callers decide what the key means, and they deliberately disagree. Live
// surfaces pass a session id, so every agent gets its own creature. The den
// passes a branch, so a creature is earned once per branch however many agents
// pass through it.
package identity

import "fmt"

type Rarity int

const (
	Common Rarity = iota
	Uncommon
	Rare
	Legendary
	Mythic
)

var rarityNames = [...]string{"common", "uncommon", "rare", "legendary", "mythic"}

func (r Rarity) String() string {
	if int(r) < len(rarityNames) {
		return rarityNames[r]
	}
	return "unknown"
}

// Stars is the compact rarity marker shown beside a creature.
func (r Rarity) Stars() string {
	marks := ""
	for index := 0; index <= int(r); index++ {
		marks += "✦"
	}
	return marks
}

// weights are per ten-thousand so the table stays exact in integer maths.
// Tuned so a legendary lands roughly one key in thirty.
var weights = [...]int{6000, 2500, 1100, 350, 50}

// Species is one creature: a name, its band, and the frame fragments bracketing its face.
type Species struct {
	Name   string
	Prefix string
	Suffix string
	Rarity Rarity
}

var species = []Species{
	// The original roster, now spread across bands.
	{"otter", "(", ")", Common},
	{"cat", "/", "\\", Common},
	{"fox", "{", "}", Common},
	{"frog", "[", "]", Common},
	{"bat", "^", "^", Common},
	{"mouse", ".(", ").", Common},
	{"crab", ">(", ")<", Common},
	{"moth", "<", ">", Common},
	{"wren", "v(", ")v", Common},
	{"vole", ",(", "),", Common},
	{"toad", "[(", ")]", Common},
	{"ant", ":(", "):", Common},
	{"snail", "@(", ")@", Common},
	{"carp", "-[", "]-", Common},

	{"rabbit", "(\\", "/)", Uncommon},
	{"gecko", "-(", ")-", Uncommon},
	{"owl", "((", "))", Uncommon},
	{"bear", "o(", ")o", Uncommon},
	{"beetle", "+(", ")+", Uncommon},
	{"heron", "|(", ")|", Uncommon},
	{"shrew", "'(", ")'", Uncommon},
	{"hare", "(/", "\\)", Uncommon},
	{"newt", "~[", "]~", Uncommon},
	{"pike", "=[", "]=", Uncommon},
	{"wasp", "}(", "){", Uncommon},
	{"slug", "%(", ")%", Uncommon},

	{"squid", "*(", ")*", Rare},
	{"koi", "=(", ")=", Rare},
	{"lynx", "/(", ")\\", Rare},
	{"seal", "o[", "]o", Rare},
	{"crow", "\\(", ")/", Rare},
	{"ray", "<[", "]>", Rare},
	{"stag", "Y(", ")Y", Rare},
	{"wolf", "/[", "]\\", Rare},

	{"axolotl", "~(", ")~", Legendary},
	{"kelpie", "~{", "}~", Legendary},
	{"selkie", "o{", "}o", Legendary},
	{"kraken", "*[", "]*", Legendary},
	{"basilisk", "§(", ")§", Legendary},

	{"wyrm", "<=", "=>", Mythic},
	{"phoenix", "*~", "~*", Mythic},
}

// byRarity indexes the roster so a band can be sampled without scanning.
var byRarity = func() map[Rarity][]Species {
	grouped := map[Rarity][]Species{}
	for _, one := range species {
		grouped[one.Rarity] = append(grouped[one.Rarity], one)
	}
	return grouped
}()

// Common and uncommon creatures take a hue hashed independently of species, so
// two live agents on the same creature still read apart. The colours reserved
// for the higher bands are deliberately absent here, so an ordinary creature can
// never be mistaken for a rare one.
var palette = []int{173, 208, 114, 203, 187, 218, 79, 179, 156}

// A rare creature wears its band. This is what makes rarity legible at a glance
// rather than a star count nobody decodes, and the colours match the ones the
// rarity stars already use.
var bandColor = map[Rarity]int{Rare: 81, Legendary: 141, Mythic: 213}

// shinyOdds is an independent roll, so a shiny common is its own kind of find.
const shinyOdds = 128

// Pet is a fully resolved creature, ready to render.
type Pet struct {
	Species
	Color int
	Shiny bool
}

// Key identifies a creature in the den. Shininess is part of the identity
// because catching a shiny is a separate event worth recording.
func (p Pet) Key() string {
	if p.Shiny {
		return p.Name + "#shiny"
	}
	return p.Name
}

func (p Pet) Label() string {
	if p.Shiny {
		return p.Name + " ✨"
	}
	return p.Name
}

// Hash folds a string into a stable integer, matching the original shell implementation.
//
// It walks bytes rather than runes so the result never depends on the caller's locale.
func Hash(text string) int {
	value := 7
	for index := 0; index < len(text); index++ {
		value = (value*31 + int(text[index])) % 1000003
	}
	return value
}

// bandFor walks the weight table, so the distribution is exact rather than sampled.
func bandFor(roll int) Rarity {
	total := 0
	for _, weight := range weights {
		total += weight
	}
	point := roll % total
	for band, weight := range weights {
		if point < weight {
			return Rarity(band)
		}
		point -= weight
	}
	return Common
}

// For returns the creature belonging to a key: a session id, a branch, anything stable.
//
// The band is chosen first so rarity stays on its intended distribution no
// matter how many creatures a pack adds to any one band.
func For(key string) Pet {
	band := bandFor(Hash("band:" + key))
	pool := byRarity[band]
	if len(pool) == 0 {
		pool = byRarity[Common]
	}
	color, reserved := bandColor[band]
	if !reserved {
		color = palette[Hash("hue:"+key)%len(palette)]
	}
	return Pet{
		Species: pool[Hash(key)%len(pool)],
		Color:   color,
		Shiny:   Hash("shiny:"+key)%shinyOdds == 0,
	}
}

// All returns the full roster, for the den's completion view.
func All() []Species {
	roster := make([]Species, len(species))
	copy(roster, species)
	return roster
}

// CountByRarity reports how many creatures exist in each band.
func CountByRarity() map[Rarity]int {
	counts := map[Rarity]int{}
	for _, one := range species {
		counts[one.Rarity]++
	}
	return counts
}

// Validate guards the roster against duplicates and frames too wide to render.
func Validate() error {
	seen := map[string]bool{}
	for _, one := range species {
		if seen[one.Name] {
			return fmt.Errorf("duplicate species %q", one.Name)
		}
		seen[one.Name] = true
		if one.Prefix == "" || one.Suffix == "" {
			return fmt.Errorf("species %q has an empty frame", one.Name)
		}
		if len(one.Prefix) > 3 || len(one.Suffix) > 3 {
			return fmt.Errorf("species %q has a frame wider than 3 characters", one.Name)
		}
	}
	for band := Common; band <= Mythic; band++ {
		if len(byRarity[band]) == 0 {
			return fmt.Errorf("no species in the %s band", band)
		}
	}
	return nil
}
