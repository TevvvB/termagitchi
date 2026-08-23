package main

import (
	"fmt"
	"os"

	"github.com/TevvvB/parallel-harness-pets/internal/harness"
)

func installCommand(action string, args []string) {
	remove := action == "uninstall"
	binary, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot resolve own path:", err)
		os.Exit(1)
	}

	requested := flagValue(args, "harness", "")
	var targets []harness.Name
	switch requested {
	case "", "all":
		targets = harness.Detect()
	default:
		targets = []harness.Name{harness.Name(requested)}
	}

	for _, target := range targets {
		var err error
		switch target {
		case harness.Claude:
			err = harness.InstallClaude(binary, remove)
		case harness.Codex:
			err = harness.InstallCodex(binary, remove)
		case harness.Tmux, harness.Shell:
			continue
		default:
			err = fmt.Errorf("unknown harness %q", target)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", target, err)
			os.Exit(1)
		}
		fmt.Printf("%s %s\n", map[bool]string{true: "removed from", false: "installed into"}[remove], target)
	}

	if remove {
		fmt.Println("\nIf you added the tmux or shell snippets by hand, remove those too.")
		return
	}

	// Codex will not run a new or changed hook until it has been trusted, and nothing
	// anywhere says so -- an untrusted hook is silent, and `codex doctor` does not mention
	// hooks at all. Verified on 0.149.0: identical config fires only with
	// --dangerously-bypass-hook-trust.
	for _, target := range targets {
		if target != harness.Codex {
			continue
		}
		fmt.Println("\nCodex will not run these until it trusts them. It asks on the next")
		fmt.Println("session start after a hook changes. Until you accept, they do nothing")
		fmt.Println("and nothing tells you so.")
	}

	// The snippets are configuration, and ending on them leaves somebody with
	// nothing to try. One creature in one repo is also not the point, so send
	// them to a second worktree, which is where this stops looking like a
	// status line and starts looking like the thing it is.
	switch requested {
	case "tmux":
		fmt.Print("\n" + harness.TmuxSnippet(binary))
		return
	case "shell":
		fmt.Print("\n" + harness.ShellSnippet(binary))
		return
	}

	if len(targets) == 0 {
		fmt.Println("No Claude Code or Codex config directory found, so nothing was wired up.")
		fmt.Println("If one of them is installed but has not been run yet, name it directly:")
		fmt.Println("\n  pets install --harness=claude")
	} else {
		fmt.Println("\nStart a new session to see it. Agents read hooks at startup.")
	}

	fmt.Println("\nThen open a worktree, and a creature hatches for it:")
	fmt.Println("\n  git worktree add ../my-feature -b feat/my-feature")
	fmt.Println("\n  pets party    every live worktree at once")
	fmt.Println("  pets den      what you have collected")
	fmt.Println("\nFor tmux or a shell prompt, which work with any agent at all:")
	fmt.Println("\n  pets install --harness=tmux")
	fmt.Println("  pets install --harness=shell")
}
