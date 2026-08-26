package release

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"0.1.2", "0.1.1", true},
		{"0.2.0", "0.1.9", true},
		{"1.0.0", "0.9.9", true},
		{"0.1.1", "0.1.1", false},
		{"0.1.0", "0.1.1", false},
		{"v0.1.2", "0.1.1", true},
		{"0.1.2", "v0.1.1", true},
		// A local build reports "dev", which must never trigger a notice.
		{"0.1.2", "dev", false},
		{"", "0.1.1", false},
		{"garbage", "0.1.1", false},
		// Numeric comparison, not lexical: 10 is above 9.
		{"0.1.10", "0.1.9", true},
		{"0.10.0", "0.9.0", true},
		// A pre-release suffix compares on its numbers alone.
		{"0.1.2-rc1", "0.1.1", true},
	}
	for _, testCase := range cases {
		if got := Newer(testCase.latest, testCase.current); got != testCase.want {
			t.Errorf("Newer(%q, %q) = %v, want %v",
				testCase.latest, testCase.current, got, testCase.want)
		}
	}
}

// Switched off means no network and no notice, whatever is cached.
func TestCheckRespectsTheOptOut(t *testing.T) {
	dir := t.TempDir()
	writeCache(dir, "9.9.9", time.Now())
	if got := Check(dir, "0.1.1", false, time.Now()); got != "" {
		t.Errorf("Check returned %q with checking disabled", got)
	}
}

// A fresh cache must answer without touching the network, which is what keeps
// this to one request a day rather than one per command.
func TestCheckUsesAFreshCache(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeCache(dir, "9.9.9", now)
	if got := Check(dir, "0.1.1", true, now); got != "9.9.9" {
		t.Errorf("Check = %q, want the cached 9.9.9", got)
	}
	// Same cache, but old enough that it would go to the network instead.
	if _, fresh := readCache(dir, now.Add(25*time.Hour)); fresh {
		t.Error("a day-old cache still reported as fresh")
	}
}

func TestCheckSaysNothingWhenCurrent(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeCache(dir, "0.1.1", now)
	if got := Check(dir, "0.1.1", true, now); got != "" {
		t.Errorf("Check = %q on an up-to-date install, want nothing", got)
	}
}

// A local build has no version to compare, so it must not reach the network.
func TestCheckSkipsUnparseableCurrent(t *testing.T) {
	dir := t.TempDir()
	if got := Check(dir, "dev", true, time.Now()); got != "" {
		t.Errorf("Check = %q for a dev build", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "latest")); err == nil {
		t.Error("a dev build still wrote a cache, so it must have tried the network")
	}
}

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeCache(dir, "0.2.0", now)
	version, fresh := readCache(dir, now)
	if version != "0.2.0" || !fresh {
		t.Errorf("readCache = %q/%v, want 0.2.0/true", version, fresh)
	}
}

func TestReadCacheOnGarbage(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "latest"), []byte("nonsense"), 0o644)
	if _, fresh := readCache(dir, time.Now()); fresh {
		t.Error("garbage read as a fresh cache")
	}
}

func TestCommandMatchesTheInstall(t *testing.T) {
	brew := "/opt/homebrew/Caskroom/termagitchi/0.1.1/pets"
	if got := Command(brew); got != "brew upgrade termagitchi" {
		t.Errorf("Command(%q) = %q", brew, got)
	}
	manual := "/home/someone/.local/bin/pets"
	if got := Command(manual); got == "brew upgrade termagitchi" {
		t.Errorf("Command(%q) suggested brew for a manual install", manual)
	}
}
