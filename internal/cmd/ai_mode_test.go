package cmd

import (
	"os"
	"testing"
)

func stubProcessTree(t *testing.T, cmdlines map[int]string, ppids map[int]int, startPid int) *int {
	t.Helper()
	parentPid := os.Getppid()
	calls := 0
	old := processInfoFn
	t.Cleanup(func() { processInfoFn = old })
	processInfoFn = func(pid int) (string, int) {
		calls++
		if pid == parentPid {
			pid = startPid
		}
		return cmdlines[pid], ppids[pid]
	}
	return &calls
}

func stubStdout(t *testing.T, isTTY bool) {
	t.Helper()
	old := stdoutIsTerminal
	t.Cleanup(func() { stdoutIsTerminal = old })
	stdoutIsTerminal = func() bool { return isTTY }
}

func stubAIModeFlag(t *testing.T, v bool) {
	t.Helper()
	old := aiModeFlag
	t.Cleanup(func() { aiModeFlag = old })
	SetAIMode(v)
}

func TestIsAIMode_defaults_false(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "")
	stubAIModeFlag(t, false)
	stubStdout(t, false)
	stubProcessTree(t, map[int]string{100: "/bin/zsh"}, map[int]int{100: 1}, 100)

	// act
	got := IsAIMode()

	// assert
	if got {
		t.Errorf("expected AI mode OFF by default, got ON")
	}
}

func TestIsAIMode_env_var_1_enables(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "1")
	stubAIModeFlag(t, false)
	stubStdout(t, false)
	stubProcessTree(t, map[int]string{100: "/bin/zsh"}, map[int]int{100: 1}, 100)

	// act
	got := IsAIMode()

	// assert
	if !got {
		t.Errorf("expected WARD_AI_MODE=1 to enable AI mode")
	}
}

func TestIsAIMode_env_var_1_enables_with_tty(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "1")
	stubAIModeFlag(t, false)
	stubStdout(t, true)
	stubProcessTree(t, map[int]string{100: "/bin/zsh"}, map[int]int{100: 1}, 100)

	// act
	got := IsAIMode()

	// assert
	if !got {
		t.Errorf("expected WARD_AI_MODE=1 to win over a terminal stdout")
	}
}

func TestIsAIMode_env_var_0_disables(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "0")
	stubAIModeFlag(t, false)
	stubStdout(t, false)
	stubProcessTree(t, map[int]string{100: "/Users/x/.opencode/bin/opencode"}, map[int]int{100: 1}, 100)

	// act
	got := IsAIMode()

	// assert
	if got {
		t.Errorf("expected WARD_AI_MODE=0 to keep AI mode OFF under an agent ancestor")
	}
}

func TestIsAIMode_flag_enables(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "0")
	stubAIModeFlag(t, true)
	stubStdout(t, true)
	stubProcessTree(t, map[int]string{100: "/bin/zsh"}, map[int]int{100: 1}, 100)

	// act
	got := IsAIMode()

	// assert
	if !got {
		t.Errorf("expected --ai-mode flag to win over WARD_AI_MODE=0")
	}
}

func TestIsAIMode_flag_enables_without_env(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "")
	stubAIModeFlag(t, true)
	stubStdout(t, false)
	stubProcessTree(t, map[int]string{100: "/bin/zsh"}, map[int]int{100: 1}, 100)

	// act
	got := IsAIMode()

	// assert
	if !got {
		t.Errorf("expected --ai-mode flag to enable AI mode")
	}
}

func TestIsAIMode_detects_opencode_ancestor(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "")
	stubAIModeFlag(t, false)
	stubStdout(t, false)
	stubProcessTree(t, map[int]string{
		100: "/bin/zsh",
		200: "/Users/x/.opencode/bin/opencode",
	}, map[int]int{100: 200, 200: 1}, 100)

	// act
	got := IsAIMode()

	// assert
	if !got {
		t.Errorf("expected AI mode ON under an opencode ancestor with piped stdout")
	}
}

func TestIsAIMode_tty_skips_detection(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "")
	stubAIModeFlag(t, false)
	stubStdout(t, true)
	stubProcessTree(t, map[int]string{
		100: "/bin/zsh",
		200: "/Users/x/.opencode/bin/opencode",
	}, map[int]int{100: 200, 200: 1}, 100)

	// act
	got := IsAIMode()

	// assert
	if got {
		t.Errorf("expected a human at a terminal not to be put in AI mode")
	}
}

func TestIsAIMode_detects_claude_bin_symlink(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "")
	stubAIModeFlag(t, false)
	stubStdout(t, false)
	stubProcessTree(t, map[int]string{
		100: "/bin/zsh",
		200: "node /x/node_modules/.bin/claude",
	}, map[int]int{100: 200, 200: 1}, 100)

	// act
	got := IsAIMode()

	// assert
	if !got {
		t.Errorf("expected AI mode ON under a node wrapper of claude")
	}
}

func TestIsAIMode_detects_claude_path_segment(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "")
	stubAIModeFlag(t, false)
	stubStdout(t, false)
	stubProcessTree(t, map[int]string{
		100: "/bin/zsh",
		200: "node /opt/claude/bin/x.js",
	}, map[int]int{100: 200, 200: 1}, 100)

	// act
	got := IsAIMode()

	// assert
	if !got {
		t.Errorf("expected AI mode ON when a path segment names claude")
	}
}

func TestIsAIMode_no_false_positive_on_agent_prefixed_dir(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "")
	stubAIModeFlag(t, false)
	stubStdout(t, false)
	stubProcessTree(t, map[int]string{
		100: "/Users/x/workspace/opencode-brl-cost/bin/app",
	}, map[int]int{100: 1}, 100)

	// act
	got := IsAIMode()

	// assert
	if got {
		t.Errorf("expected no false positive for a directory merely prefixed with an agent name")
	}
}

func TestIsAIMode_detects_agent_three_levels_up(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "")
	stubAIModeFlag(t, false)
	stubStdout(t, false)
	stubProcessTree(t, map[int]string{
		100: "/bin/zsh",
		200: "/usr/local/bin/bash",
		300: "/Users/x/.opencode/bin/opencode",
		400: "/bin/launchd",
	}, map[int]int{100: 200, 200: 300, 300: 400, 400: 1}, 100)

	// act
	got := IsAIMode()

	// assert
	if !got {
		t.Errorf("expected AI mode ON when the agent sits three levels above ward")
	}
}

func TestIsAIMode_walk_stops_after_sixteen_levels(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "")
	stubAIModeFlag(t, false)
	stubStdout(t, false)
	cmdlines := map[int]string{}
	ppids := map[int]int{}
	for pid := 100; pid <= 130; pid++ {
		cmdlines[pid] = "/bin/zsh"
		ppids[pid] = pid + 1
	}
	calls := stubProcessTree(t, cmdlines, ppids, 100)

	// act
	got := IsAIMode()

	// assert
	if got {
		t.Errorf("expected AI mode OFF when no agent is within 16 levels")
	}
	if *calls != 16 {
		t.Errorf("expected the walk to stop after 16 levels, got %d lookups", *calls)
	}
}

func TestIsAIMode_cyclic_chain_terminates(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "")
	stubAIModeFlag(t, false)
	stubStdout(t, false)
	calls := stubProcessTree(t,
		map[int]string{4242: "/bin/zsh"},
		map[int]int{4242: 4242},
		4242)

	// act
	got := IsAIMode()

	// assert
	if got {
		t.Errorf("expected AI mode OFF for a cyclic parent chain")
	}
	if *calls != 16 {
		t.Errorf("expected the walk to stop after 16 levels, got %d lookups", *calls)
	}
}

func TestIsAIMode_stops_when_ancestor_is_unknown(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "")
	stubAIModeFlag(t, false)
	stubStdout(t, false)
	calls := stubProcessTree(t,
		map[int]string{100: "/bin/zsh", 200: "/usr/local/bin/bash"},
		map[int]int{100: 200, 200: 300},
		100)

	// act
	got := IsAIMode()

	// assert
	if got {
		t.Errorf("expected AI mode OFF when the parent chain cannot be inspected")
	}
	if *calls != 3 {
		t.Errorf("expected the walk to stop at the unknown process, got %d lookups", *calls)
	}
}

func TestAgentCommandName(t *testing.T) {
	cases := []struct {
		name    string
		cmdline string
		want    string
	}{
		{"bare opencode", "opencode", "opencode"},
		{"opencode path", "/Users/x/.opencode/bin/opencode", "opencode"},
		{"opencode with args", "/usr/local/bin/opencode run --model m", "opencode"},
		{"openclaw", "/usr/local/bin/openclaw", "openclaw"},
		{"claude symlink", "node /x/node_modules/.bin/claude", "claude"},
		{"claude segment", "node /opt/claude/bin/x.js", "claude"},
		{"codex", "/usr/local/bin/codex", "codex"},
		{"cursor-agent", "/usr/local/bin/cursor-agent", "cursor-agent"},
		{"aider", "/usr/local/bin/aider", "aider"},
		{"agent prefixed dir", "/Users/x/workspace/opencode-brl-cost/bin/app", ""},
		{"plain shell", "/bin/zsh", ""},
		{"empty", "", ""},
		{"agent name inside a word", "/usr/local/bin/opencodex", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// act
			got := agentCommandName(tc.cmdline)

			// assert
			if got != tc.want {
				t.Errorf("agentCommandName(%q) = %q, want %q", tc.cmdline, got, tc.want)
			}
		})
	}
}
