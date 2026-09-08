// Package agents is the register of who is currently working where.
//
// A pet belongs to an agent, not to a directory, so something has to remember
// which agent is in which den and when it was last heard from. That is this.
//
// Every agent writes only its own file, named for its session, so two agents in
// one worktree never contend for a write and no locking is needed. The register
// is disposable: a missing or torn file costs a row in a list, never an error.
package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// Heartbeat is how often an agent bothers to re-record itself. The status
	// line calls in roughly once a second in every session; writing that often
	// would put pointless I/O on the one path that must stay cheap.
	Heartbeat = 10 * time.Second
	// StaleAfter is when an agent stops counting as live. An agent that exits
	// never gets to say so, so silence is the only signal available.
	StaleAfter = 90 * time.Second
	// Forgotten is when a silent agent stops being listed at all, so a machine
	// left running for a week does not accumulate ghosts.
	Forgotten = 12 * time.Hour
)

// Record is one agent, pinned to the den it is working in.
type Record struct {
	// Session is the harness's own id for this agent. It is the key the pet is
	// derived from, which is what pins a creature to an agent.
	Session string
	// Den is the repo-and-worktree key, so agents group without consulting git.
	Den string
	// Root is the worktree path, kept for display. It is deliberately not used
	// to decide whether an agent still exists: a worktree can be deleted out
	// from under a session that is very much still running.
	Root string
	// Name is the harness's human label for the session. It is what makes a
	// list of agents readable rather than a column of UUIDs.
	Name   string
	Branch string
	Seen   time.Time
	// Since is when this agent last changed den. Seen says the agent is alive;
	// Since says how long it has been here, which is a different question once
	// an agent can move between worktrees.
	Since time.Time
	// Context is the context window's fill percentage; zero means unknown, since only the status line reports it.
	Context int
	// ContextAt is when Context last changed to a new non-zero value.
	ContextAt time.Time
	// PrevContext and PrevContextAt are the prior sample, used to estimate time-to-full.
	PrevContext   int
	PrevContextAt time.Time
}

func (r Record) Stale(now time.Time) bool { return now.Sub(r.Seen) >= StaleAfter }

// JustArrived reports an agent that has only recently moved into this den, so a
// listing can show travel rather than presenting a newcomer as a fixture.
func (r Record) JustArrived(now time.Time) bool {
	return !r.Since.IsZero() && now.Sub(r.Since) < time.Minute
}

// Label is what to show a human: the session's own name when it has one, and a
// short slice of the id when it does not, so the column is never blank.
func (r Record) Label() string {
	if name := strings.TrimSpace(r.Name); name != "" {
		return name
	}
	if len(r.Session) > 8 {
		return r.Session[:8]
	}
	return r.Session
}

// ContextETA estimates time until the window is full from the last two samples.
//
// Returns 0 when burn rate is unknown, flat, or negative (a compact), or when the
// estimate would be too noisy or too far out to be useful on a status line.
func (r Record) ContextETA() time.Duration {
	if r.Context <= 0 || r.Context >= 100 {
		return 0
	}
	if r.PrevContext <= 0 || r.ContextAt.IsZero() || r.PrevContextAt.IsZero() {
		return 0
	}
	if r.Context <= r.PrevContext {
		return 0
	}
	elapsed := r.ContextAt.Sub(r.PrevContextAt)
	if elapsed < 30*time.Second {
		return 0
	}
	rate := float64(r.Context-r.PrevContext) / elapsed.Seconds()
	if rate <= 0 {
		return 0
	}
	secs := float64(100-r.Context) / rate
	eta := time.Duration(secs * float64(time.Second))
	if eta < time.Minute {
		return time.Minute
	}
	if eta > 3*time.Hour {
		return 0
	}
	return eta.Round(time.Minute)
}

// Get loads one agent by session. Missing files are a miss, never an error callers must handle.
func Get(stateDir, session string) (Record, bool) {
	if session == "" {
		return Record{}, false
	}
	record, err := read(filepath.Join(dir(stateDir), fileName(session)))
	if err != nil {
		return Record{}, false
	}
	return record, true
}

func dir(stateDir string) string { return filepath.Join(stateDir, "agents") }

// fileName keeps a session id safe to use as a filename on every platform,
// for the same reason the worktree cache key does.
func fileName(session string) string {
	var flat strings.Builder
	for _, char := range session {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9', char == '-':
			flat.WriteRune(char)
		default:
			flat.WriteRune('_')
		}
	}
	return flat.String() + ".agent"
}

// Touch records an agent as alive, skipping the write when the existing record
// is recent enough. The status line calls this once a second per session.
func Touch(stateDir string, record Record, now time.Time) error {
	if record.Session == "" {
		return nil
	}
	path := filepath.Join(dir(stateDir), fileName(record.Session))
	record.Since = now
	contextChanged := false
	if existing, err := read(path); err == nil {
		// A hook carries no context window, and must not reset what the status line knew.
		if record.Context == 0 {
			record.Context = existing.Context
			record.ContextAt = existing.ContextAt
			record.PrevContext = existing.PrevContext
			record.PrevContextAt = existing.PrevContextAt
		} else if record.Context != existing.Context {
			// A new fill percentage is worth writing even inside the heartbeat window:
			// otherwise the burn-rate sample that feeds ContextETA never lands.
			contextChanged = true
			if existing.Context > 0 {
				record.PrevContext = existing.Context
				if !existing.ContextAt.IsZero() {
					record.PrevContextAt = existing.ContextAt
				} else {
					record.PrevContextAt = existing.Seen
				}
			} else {
				record.PrevContext = existing.PrevContext
				record.PrevContextAt = existing.PrevContextAt
			}
			record.ContextAt = now
		} else {
			record.ContextAt = existing.ContextAt
			record.PrevContext = existing.PrevContext
			record.PrevContextAt = existing.PrevContextAt
			if record.ContextAt.IsZero() {
				record.ContextAt = now
			}
		}
		if existing.Den == record.Den {
			// Same den, so the arrival time carries over rather than being reset
			// by every heartbeat.
			record.Since = existing.Since
			if !contextChanged && now.Sub(existing.Seen) < Heartbeat {
				return nil
			}
		}
		// A changed den is written immediately whatever the heartbeat says: the
		// move is the interesting event, and delaying it loses the arrival time.
	} else if record.Context > 0 {
		record.ContextAt = now
	}
	if err := os.MkdirAll(dir(stateDir), 0o755); err != nil {
		return err
	}
	record.Seen = now
	var out strings.Builder
	fmt.Fprintf(&out, "session=%s\nden=%s\nroot=%s\nbranch=%s\nname=%s\nts=%d\nsince=%d\ncontext=%d\ncontext_ts=%d\nprev_context=%d\nprev_context_ts=%d\n",
		record.Session, record.Den, record.Root, record.Branch,
		// A newline in a session name would forge a second field on read.
		strings.ReplaceAll(record.Name, "\n", " "), record.Seen.Unix(), record.Since.Unix(),
		record.Context, unixOrZero(record.ContextAt), record.PrevContext, unixOrZero(record.PrevContextAt))
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(out.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// Beat records an agent as still alive without claiming to know where it is.
//
// An agent that steps outside a git worktree is still running, and coupling
// registration to location made it vanish from every listing after 90 seconds.
// The last known den is preserved rather than cleared, so the agent stays where
// you last saw it instead of blinking out and back.
func Beat(stateDir, session string, now time.Time) error {
	if session == "" {
		return nil
	}
	existing, err := read(filepath.Join(dir(stateDir), fileName(session)))
	if err != nil {
		// Nothing to keep alive: an agent that has never been in a worktree has
		// no den to show and nothing worth recording.
		return nil
	}
	return Touch(stateDir, existing, now)
}

func read(path string) (Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	record := Record{}
	for _, line := range strings.Split(string(data), "\n") {
		name, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch name {
		case "session":
			record.Session = value
		case "den":
			record.Den = value
		case "root":
			record.Root = value
		case "branch":
			record.Branch = value
		case "name":
			record.Name = value
		case "ts":
			seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil {
				return Record{}, err
			}
			record.Seen = time.Unix(seconds, 0)
		case "context":
			percent, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return Record{}, err
			}
			record.Context = percent
		case "context_ts":
			seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil {
				return Record{}, err
			}
			if seconds > 0 {
				record.ContextAt = time.Unix(seconds, 0)
			}
		case "prev_context":
			percent, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return Record{}, err
			}
			record.PrevContext = percent
		case "prev_context_ts":
			seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil {
				return Record{}, err
			}
			if seconds > 0 {
				record.PrevContextAt = time.Unix(seconds, 0)
			}
		case "since":
			seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil {
				return Record{}, err
			}
			record.Since = time.Unix(seconds, 0)
		}
	}
	if record.Session == "" {
		return Record{}, fmt.Errorf("no session in %s", path)
	}
	return record, nil
}

// All returns every agent still worth showing, most recently seen first.
//
// Silence is the only thing that retires an agent. Pruning on a missing
// directory used to delete live agents whose worktree had just been removed,
// which is a rendering concern rather than an existence one.
func All(stateDir string, now time.Time) []Record {
	entries, err := os.ReadDir(dir(stateDir))
	if err != nil {
		return nil
	}
	var found []Record
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".agent") {
			continue
		}
		path := filepath.Join(dir(stateDir), entry.Name())
		record, err := read(path)
		if err != nil {
			os.Remove(path)
			continue
		}
		if now.Sub(record.Seen) >= Forgotten {
			os.Remove(path)
			continue
		}
		found = append(found, record)
	}
	sort.Slice(found, func(first, second int) bool {
		return found[first].Seen.After(found[second].Seen)
	})
	return found
}

// Representative picks the agent that should stand for a den.
//
// A den always shows a creature, but when somebody is actually working there
// the creature ought to be theirs rather than one derived from the branch. A
// live agent always outranks a silent one, however recently the silent one
// spoke, because a den's face should belong to whoever is still in it.
//
// It does not assume the caller sorted anything, so the rule survives a change
// to how the register is ordered.
func Representative(records []Record, now time.Time) (Record, bool) {
	var best Record
	found := false
	for _, record := range records {
		switch {
		case !found:
			best, found = record, true
		case best.Stale(now) && !record.Stale(now):
			best = record
		case best.Stale(now) == record.Stale(now) && record.Seen.After(best.Seen):
			best = record
		}
	}
	return best, found
}

// InDen returns the agents working in one den, most recently seen first.
func InDen(stateDir, den string, now time.Time) []Record {
	var found []Record
	for _, record := range All(stateDir, now) {
		if record.Den == den {
			found = append(found, record)
		}
	}
	return found
}