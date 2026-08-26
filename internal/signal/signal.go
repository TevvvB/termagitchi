// Package signal gathers everything the pet knows about a worktree.
//
// This is the expensive half of the design and never runs on the render path.
package signal

import (
	"bufio"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/TevvvB/termagitchi/internal/config"
	"github.com/TevvvB/termagitchi/internal/gitrepo"
	"github.com/TevvvB/termagitchi/internal/state"
)

// Collect runs every enabled signal and returns the cache the renderer will read.
func Collect(repo gitrepo.Repo, settings config.Config, now time.Time) state.State {
	collected := state.State{
		External: map[string]int{}, Stamp: now,
		Root: repo.Root, Branch: repo.Branch,
	}
	trunk := gitrepo.DefaultBranch(repo.Root, settings.Branch.Default)

	if settings.SignalEnabled("dirty") {
		collected.Dirty = countLines(git(repo.Root, "status", "--porcelain"))
	}
	if settings.SignalEnabled("unpushed") {
		collected.Unpushed = countNumber(git(repo.Root, "rev-list", "--count", "@{upstream}..HEAD"))
		if collected.Unpushed == 0 {
			collected.Unpushed = countNumber(git(repo.Root, "rev-list", "--count", trunk+"..HEAD"))
		}
	}
	if settings.SignalEnabled("behind") {
		collected.Behind = countNumber(git(repo.Root, "rev-list", "--count", "HEAD.."+trunk))
	}
	if settings.Signals.Migrations.Enabled {
		collected.Migrations = MigrationHeads(repo.Root, settings.Signals.Migrations.Paths)
	}
	for name, value := range runExternal(repo.Root, settings.Signals.External) {
		collected.External[name] = value
	}
	return collected
}

// Earned reports whether a worktree has done enough to enter the den.
//
// Identity comes from a name the user chooses, so a scripted checkout -b loop
// could otherwise mine for mythics. Requiring one commit prices that at a commit
// per roll and keeps the den a record of branches where work actually happened.
func Earned(repo gitrepo.Repo, settings config.Config) bool {
	trunk := gitrepo.DefaultBranch(repo.Root, settings.Branch.Default)
	if gitrepo.IsTrunk(trunk, repo.Branch) {
		return true
	}
	if ahead := git(repo.Root, "rev-list", "--count", trunk+"..HEAD"); ahead != "" {
		return countNumber(ahead) > 0
	}
	// No trunk to compare against at all. Any commit is the best signal left,
	// which still beats never letting this repo collect anything.
	return countNumber(git(repo.Root, "rev-list", "--count", "HEAD")) > 0
}

func git(root string, args ...string) string {
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func countLines(out string) int {
	count := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func countNumber(out string) int {
	value, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil || value < 0 {
		return 0
	}
	return value
}

var (
	revisionPattern     = regexp.MustCompile(`(?m)^revision\s*[:=].*?['"]([^'"]+)['"]`)
	downRevisionPattern = regexp.MustCompile(`(?m)^down_revision\s*[:=].*?['"]([^'"]+)['"]`)
	skipDirectories     = map[string]bool{
		".git": true, "node_modules": true, "vendor": true, ".venv": true,
		"venv": true, "target": true, "dist": true, "build": true, ".tox": true,
	}
)

// MigrationHeads reports the worst single migration tree's head count.
//
// Each tree should have exactly one head, so summing across trees would cry wolf
// on a healthy repo that simply has two of them.
func MigrationHeads(root string, suffixes []string) int {
	worst := 0
	for _, directory := range findMigrationDirs(root, suffixes) {
		if heads := headsIn(directory); heads > worst {
			worst = heads
		}
	}
	return worst
}

// findMigrationDirs matches a trailing path suffix at any depth, so a repo with
// alembic/versions at its root counts just as much as one nested in a monorepo.
func findMigrationDirs(root string, suffixes []string) []string {
	var found []string
	filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return nil
		}
		if path != root && skipDirectories[entry.Name()] {
			return filepath.SkipDir
		}
		relative := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		for _, suffix := range suffixes {
			if relative == suffix || strings.HasSuffix(relative, "/"+suffix) {
				found = append(found, path)
				return filepath.SkipDir
			}
		}
		return nil
	})
	return found
}

func headsIn(directory string) int {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return 0
	}
	revisions := map[string]bool{}
	parents := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".py") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			continue
		}
		for _, match := range revisionPattern.FindAllStringSubmatch(string(data), -1) {
			revisions[match[1]] = true
		}
		for _, match := range downRevisionPattern.FindAllStringSubmatch(string(data), -1) {
			parents[match[1]] = true
		}
	}
	heads := 0
	for revision := range revisions {
		if !parents[revision] {
			heads++
		}
	}
	return heads
}

// runExternal executes user-supplied probes and reads key=value lines from stdout.
func runExternal(root string, settings config.External) map[string]int {
	results := map[string]int{}
	directory := expandHome(settings.Dir)
	if directory == "" {
		return results
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return results
	}
	timeout := time.Duration(settings.TimeoutMs) * time.Millisecond
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !runnable(entry) {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		out, err := exec.CommandContext(ctx, filepath.Join(directory, entry.Name()), root).Output()
		cancel()
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			name, value, found := strings.Cut(scanner.Text(), "=")
			if !found {
				continue
			}
			results[strings.TrimSpace(name)] = countNumber(value)
		}
	}
	return results
}

// runnable decides whether a file in the signals directory should be executed.
//
// Windows has no execute bit, so requiring one there would silently ignore
// every user-supplied signal on that platform.
func runnable(entry os.DirEntry) bool {
	if runtime.GOOS == "windows" {
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".exe", ".bat", ".cmd", ".ps1", ".com":
			return true
		}
		return false
	}
	info, err := entry.Info()
	return err == nil && info.Mode()&0o111 != 0
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}
