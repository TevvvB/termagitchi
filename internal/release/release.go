// Package release reports whether a newer version has been published.
//
// The check only ever runs from commands a person typed. Nothing on the render
// path or in a hook touches the network, and a failure of any kind is silent:
// offline, rate limited or blocked all just mean no notice appears.
package release

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	latestURL = "https://github.com/TevvvB/termagitchi/releases/latest"
	// How long a fetched answer is trusted before asking again.
	interval = 24 * time.Hour
	// Short enough that a hanging network never keeps somebody waiting.
	timeout = 2 * time.Second
)

// Newer reports whether latest is a higher version than current.
//
// A version that will not parse, such as the "dev" a local build carries,
// compares as older than nothing so it never triggers a notice.
func Newer(latest, current string) bool {
	left, ok := parse(latest)
	if !ok {
		return false
	}
	right, ok := parse(current)
	if !ok {
		return false
	}
	for index := 0; index < 3; index++ {
		if left[index] != right[index] {
			return left[index] > right[index]
		}
	}
	return false
}

func parse(version string) ([3]int, bool) {
	var parts [3]int
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	// Drop any pre-release or build suffix before comparing.
	if cut := strings.IndexAny(version, "-+"); cut >= 0 {
		version = version[:cut]
	}
	fields := strings.Split(version, ".")
	if len(fields) != 3 {
		return parts, false
	}
	for index, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil {
			return parts, false
		}
		parts[index] = value
	}
	return parts, true
}

// Check returns a newer published version, or an empty string when there is
// none, when the answer is not known yet, or when checking is switched off.
func Check(stateDir, current string, enabled bool, now time.Time) string {
	if !enabled {
		return ""
	}
	if _, ok := parse(current); !ok {
		return ""
	}
	latest, fresh := readCache(stateDir, now)
	if !fresh {
		fetched, err := fetch()
		// Record the attempt either way, so a machine with no network asks once
		// a day rather than on every command.
		writeCache(stateDir, fetched, now)
		if err != nil {
			return ""
		}
		latest = fetched
	}
	if Newer(latest, current) {
		return latest
	}
	return ""
}

// fetch reads the version from the redirect that /releases/latest issues, which
// avoids the API and its rate limit for unauthenticated callers.
func fetch() (string, error) {
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Get(latestURL)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	location := response.Header.Get("Location")
	tag := location[strings.LastIndex(location, "/")+1:]
	if _, ok := parse(tag); !ok {
		return "", fmt.Errorf("unexpected release location %q", location)
	}
	return strings.TrimPrefix(tag, "v"), nil
}

func cachePath(stateDir string) string {
	return filepath.Join(stateDir, "latest")
}

func readCache(stateDir string, now time.Time) (string, bool) {
	data, err := os.ReadFile(cachePath(stateDir))
	if err != nil {
		return "", false
	}
	version, stamp := "", int64(0)
	for _, line := range strings.Split(string(data), "\n") {
		name, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch name {
		case "version":
			version = strings.TrimSpace(value)
		case "ts":
			stamp, _ = strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		}
	}
	return version, now.Sub(time.Unix(stamp, 0)) < interval
}

func writeCache(stateDir, version string, now time.Time) {
	if os.MkdirAll(stateDir, 0o755) != nil {
		return
	}
	os.WriteFile(cachePath(stateDir),
		[]byte(fmt.Sprintf("version=%s\nts=%d\n", version, now.Unix())), 0o644)
}

// Command is how this particular install updates itself, guessed from where the
// binary lives so the notice tells somebody something they can actually run.
func Command(executable string) string {
	if strings.Contains(executable, "/Caskroom/") || strings.Contains(executable, "/Cellar/") ||
		strings.Contains(executable, "/homebrew/") {
		return "brew upgrade termagitchi"
	}
	return "curl -fsSL https://raw.githubusercontent.com/TevvvB/termagitchi/main/install.sh | sh"
}
