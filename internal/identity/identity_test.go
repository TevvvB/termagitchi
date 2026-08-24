package identity

import (
	"fmt"
	"testing"
)

// Golden values for the current roster. Species have been stable since rarity
// bands landed; colours moved once after that, when rare and above started
// wearing their band. This table exists so neither changes again by accident.
func TestForIsPinned(t *testing.T) {
	cases := []struct {
		key     string
		species string
		color   int
		rarity  Rarity
		shiny   bool
	}{
		{"main", "mouse", 208, Common, false},
		{"master", "vole", 203, Common, false},
		{"develop", "vole", 79, Common, false},
		{"trunk", "wren", 114, Common, false},
		{"feat/oauth-refresh", "koi", 81, Rare, false},
		{"refactor/auth-guard", "moth", 203, Common, false},
		{"agent-task-1785874237", "snail", 114, Common, false},
		{"claude/wiki-updates", "gecko", 79, Uncommon, false},
		{"spike/wasm-build", "cat", 156, Common, false},
		{"fix/n-plus-one-query", "owl", 208, Uncommon, false},
		{"chore/bump-deps", "fox", 187, Common, false},
		{"docs/install-rewrite", "seal", 81, Rare, false},
		{"a", "crab", 187, Common, false},
	}

	for _, testCase := range cases {
		pet := For(testCase.key)
		if pet.Name != testCase.species || pet.Color != testCase.color ||
			pet.Rarity != testCase.rarity || pet.Shiny != testCase.shiny {
			t.Errorf("For(%q) = %s/%d/%s/shiny=%v, want %s/%d/%s/shiny=%v",
				testCase.key, pet.Name, pet.Color, pet.Rarity, pet.Shiny,
				testCase.species, testCase.color, testCase.rarity, testCase.shiny)
		}
	}
}

func TestForIsDeterministic(t *testing.T) {
	first := For("feat/checkout-flow")
	for attempt := 0; attempt < 100; attempt++ {
		if For("feat/checkout-flow") != first {
			t.Fatal("For is not deterministic")
		}
	}
}

// The weight table is authoritative, not the pool sizes. This is what lets packs
// add creatures to any band without inflating how often that band appears.
func TestRarityMatchesTheWeightTable(t *testing.T) {
	const sample = 20000
	counts := map[Rarity]int{}
	for index := 0; index < sample; index++ {
		counts[For(fmt.Sprintf("feat/ticket-%d-work", index)).Rarity]++
	}
	targets := map[Rarity]float64{
		Common: 60, Uncommon: 25, Rare: 11, Legendary: 3.5, Mythic: 0.5,
	}
	for band, target := range targets {
		got := float64(counts[band]) / float64(sample) * 100
		if got < target-1.5 || got > target+1.5 {
			t.Errorf("%s = %.2f%%, want %.1f%% within 1.5 points", band, got, target)
		}
	}
}

// Adding creatures to a band must not change how often that band is rolled.
func TestPoolSizeDoesNotChangeBandOdds(t *testing.T) {
	pools := CountByRarity()
	if pools[Mythic] >= pools[Common] {
		t.Skip("roster no longer has a small mythic pool to prove the point with")
	}
	const sample = 20000
	mythic := 0
	for index := 0; index < sample; index++ {
		if For(fmt.Sprintf("branch-%d", index)).Rarity == Mythic {
			mythic++
		}
	}
	rate := float64(mythic) / float64(sample) * 100
	if rate > 2 {
		t.Errorf("mythic rate %.2f%% is far above its 0.5%% weight", rate)
	}
}

func TestShinyIsIndependentOfRarity(t *testing.T) {
	const sample = 20000
	shiny, shinyCommon := 0, 0
	for index := 0; index < sample; index++ {
		pet := For(fmt.Sprintf("feat/ticket-%d-work", index))
		if pet.Shiny {
			shiny++
			if pet.Rarity == Common {
				shinyCommon++
			}
		}
	}
	rate := float64(shiny) / float64(sample)
	if rate < 0.004 || rate > 0.013 {
		t.Errorf("shiny rate %.4f, want roughly 1 in 128", rate)
	}
	if shinyCommon == 0 {
		t.Error("no shiny commons appeared, so shininess is not independent of band")
	}
}

func TestRosterIsValid(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestKeyDistinguishesShinies(t *testing.T) {
	plain := Pet{Species: Species{Name: "koi"}}
	shiny := Pet{Species: Species{Name: "koi"}, Shiny: true}
	if plain.Key() == shiny.Key() {
		t.Error("a shiny and a plain koi share a den key, so one would overwrite the other")
	}
}

func TestForHandlesEmptyAndUnicodeBranches(t *testing.T) {
	for _, branch := range []string{"", "ünïcøde/brânch", "very/" + string(make([]byte, 300))} {
		pet := For(branch)
		if pet.Name == "" || pet.Prefix == "" {
			t.Errorf("For(%q) produced an unusable pet: %+v", branch, pet)
		}
	}
}

func TestHashIsStable(t *testing.T) {
	if Hash("main") != Hash("main") {
		t.Fatal("Hash is not stable across calls")
	}
	// Species, colour, band and shininess each need their own hash, or they correlate.
	seeds := []string{"main", "hue:main", "band:main", "shiny:main"}
	seen := map[int]bool{}
	for _, seed := range seeds {
		if seen[Hash(seed)] {
			t.Errorf("hash collision across seeds at %q, which would tie two traits together", seed)
		}
		seen[Hash(seed)] = true
	}
}

// Rarity has to be visible without decoding a star count, so anything rare or
// above wears its band's colour. Commons and uncommons keep an independent hue,
// which is what stops two live agents on the same creature reading alike.
func TestRareCreaturesWearTheirBand(t *testing.T) {
	bands := map[Rarity]int{}
	ordinary := map[int]bool{}
	for index := 0; index < 4000; index++ {
		pet := For(fmt.Sprintf("feat/ticket-%d", index))
		if pet.Rarity >= Rare {
			if seen, ok := bands[pet.Rarity]; ok && seen != pet.Color {
				t.Fatalf("%s appeared in two colours, %d and %d", pet.Rarity, seen, pet.Color)
			}
			bands[pet.Rarity] = pet.Color
			continue
		}
		ordinary[pet.Color] = true
	}
	for band, color := range bands {
		if ordinary[color] {
			t.Errorf("%s uses colour %d, which an ordinary creature can also have", band, color)
		}
	}
	if len(ordinary) < 5 {
		t.Errorf("only %d hues left for common and uncommon, too few to tell agents apart", len(ordinary))
	}
}
