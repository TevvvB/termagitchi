// Package harness wires the binary into whichever agent tools are installed.
//
// Only this package knows what a harness is. Everything else renders text.
package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Name identifies one supported surface.
type Name string

const (
	Claude Name = "claude"
	Codex  Name = "codex"
	Tmux   Name = "tmux"
	Shell  Name = "shell"
)

// ours matches commands this tool wrote, including under its former name, so a
// re-run replaces its own entries instead of stacking duplicates.
func ours(command string) bool {
	for _, marker := range []string{"parallel-harness-pets", "claude-buddy", "/pets "} {
		if strings.Contains(command, marker) {
			return true
		}
	}
	return strings.HasSuffix(command, "/pets") || command == "pets"
}

func claudeSettings() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func codexHooks() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "hooks.json")
}

// Detect reports which harnesses are actually present, so install can default
// to everything the user really has.
func Detect() []Name {
	home, _ := os.UserHomeDir()
	var found []Name
	if _, err := os.Stat(filepath.Join(home, ".claude")); err == nil {
		found = append(found, Claude)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex")); err == nil {
		found = append(found, Codex)
	}
	return found
}

func backup(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	stamp := time.Now().Format("20060102150405")
	return os.WriteFile(path+".bak-pets-"+stamp, data, 0o644)
}

func loadJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{}, nil
	}
	loaded := map[string]any{}
	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON, refusing to overwrite it", path)
	}
	return loaded, nil
}

func saveJSON(path string, value map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

// setHook drops any entry this tool previously wrote for an event, then adds one.
// Passing an empty command removes ours without adding anything back.
func setHook(settings map[string]any, event, command, matcher string) {
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	existing, _ := hooks[event].([]any)
	kept := []any{}
	for _, entry := range existing {
		group, _ := entry.(map[string]any)
		if group == nil {
			continue
		}
		inner, _ := group["hooks"].([]any)
		survivors := []any{}
		for _, item := range inner {
			hook, _ := item.(map[string]any)
			if hook == nil {
				continue
			}
			if text, _ := hook["command"].(string); !ours(text) {
				survivors = append(survivors, item)
			}
		}
		if len(survivors) > 0 {
			group["hooks"] = survivors
			kept = append(kept, group)
		}
	}
	if command != "" {
		added := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command}}}
		if matcher != "" {
			added["matcher"] = matcher
		}
		kept = append(kept, added)
	}
	if len(kept) == 0 {
		delete(hooks, event)
	} else {
		hooks[event] = kept
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}
}

// InstallClaude points the status line and hooks at the binary.
func InstallClaude(binary string, remove bool) error {
	path := claudeSettings()
	settings, err := loadJSON(path)
	if err != nil {
		return err
	}
	if err := backup(path); err != nil {
		return err
	}
	if remove {
		if line, _ := settings["statusLine"].(map[string]any); line != nil {
			if command, _ := line["command"].(string); ours(command) {
				delete(settings, "statusLine")
			}
		}
		setHook(settings, "PostToolUse", "", "")
		setHook(settings, "Stop", "", "")
		setHook(settings, "SessionStart", "", "")
	} else {
		settings["statusLine"] = map[string]any{
			"type": "command", "command": binary + " render",
			"padding": 1, "refreshInterval": 1,
		}
		setHook(settings, "PostToolUse", binary+" record", "Bash")
		setHook(settings, "Stop", binary+" quip", "")
		setHook(settings, "SessionStart", binary+" hatch", "")
	}
	return saveJSON(path, settings)
}

// InstallCodex writes user-level hooks. Codex uses the same event schema as
// Claude Code, but its status line is a fixed item list, so the pet lives in the
// terminal title there instead.
func InstallCodex(binary string, remove bool) error {
	path := codexHooks()
	settings, err := loadJSON(path)
	if err != nil {
		return err
	}
	if err := backup(path); err != nil {
		return err
	}
	if remove {
		setHook(settings, "PostToolUse", "", "")
		setHook(settings, "Stop", "", "")
		setHook(settings, "SessionStart", "", "")
	} else {
		setHook(settings, "PostToolUse", binary+" record", "Bash")
		setHook(settings, "Stop", binary+" quip", "")
		setHook(settings, "SessionStart", binary+" hatch", "")
	}
	return saveJSON(path, settings)
}

// TmuxSnippet is printed rather than written, because silently editing somebody's
// tmux config is a worse surprise than one line to paste.
//
// It passes the pane's path explicitly: tmux runs #(...) from its server's
// working directory, not the pane's, so without this every pane shows the same pet.
func TmuxSnippet(binary string) string {
	return fmt.Sprintf(`# ~/.tmux.conf
set -g status-right '#(%s render --format=tmux --cwd=#{pane_current_path})'
set -g status-interval 5
`, binary)
}

func ShellSnippet(binary string) string {
	return fmt.Sprintf(`# ~/.zshrc - shows the pet in your terminal title, any agent, no harness support needed
pets_title() { printf '\033]0;%%s\007' "$(%s render --format=title)"; }
precmd_functions+=(pets_title)
`, binary)
}
