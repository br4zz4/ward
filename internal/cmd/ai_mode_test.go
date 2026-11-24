package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const levelHelperEnv = "WARD_CMD_TEST_LEVEL_HELPER"
const levelHelperFlagEnv = "WARD_CMD_TEST_LEVEL_FLAG"
const levelHelperAgentEnv = "WARD_CMD_TEST_LEVEL_AGENT"
const levelHelperSample = "tok_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789abcdefXa9f2"

func TestAILevelHelperProcess(t *testing.T) {
	if os.Getenv(levelHelperEnv) != "1" {
		t.Skip("helper process only")
	}
	stdoutIsTerminal = func() bool { return false }
	agent := os.Getenv(levelHelperAgentEnv) == "1"
	parentPid := os.Getppid()
	processInfoFn = func(pid int) (string, int) {
		if pid == parentPid && agent {
			return "/Users/x/.opencode/bin/opencode", 1
		}
		return "/bin/zsh", 1
	}
	SetAIMode(os.Getenv(levelHelperFlagEnv) == "1")
	on := IsAIMode()
	level := AILevel()
	fmt.Printf("RESULT ai=%v level=%s val=%s\n", on, level, aiValue(levelHelperSample))
	os.Exit(0)
}

type levelRun struct {
	ai     bool
	level  string
	val    string
	stderr string
}

func runLevelHelper(t *testing.T, env, config string, flag, agent bool) levelRun {
	t.Helper()
	dir := t.TempDir()
	if config != "-" {
		if err := os.MkdirAll(filepath.Join(dir, ".ward"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".ward", "config.yaml"), []byte(config+"vaults: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := exec.Command(os.Args[0], "-test.run=^TestAILevelHelperProcess$")
	c.Dir = dir
	envs := []string{levelHelperEnv + "=1", "WARD_AI_MODE=" + env}
	if flag {
		envs = append(envs, levelHelperFlagEnv+"=1")
	}
	if agent {
		envs = append(envs, levelHelperAgentEnv+"=1")
	}
	c.Env = append(os.Environ(), envs...)
	var out, errb bytes.Buffer
	c.Stdout, c.Stderr = &out, &errb
	if err := c.Run(); err != nil {
		t.Fatalf("helper failed: %v\nstdout: %s\nstderr: %s", err, out.String(), errb.String())
	}
	var res levelRun
	res.stderr = errb.String()
	for _, line := range strings.Split(out.String(), "\n") {
		if !strings.HasPrefix(line, "RESULT ") {
			continue
		}
		rest := strings.TrimPrefix(line, "RESULT ai=")
		aiStr, rest, _ := strings.Cut(rest, " level=")
		level, val, _ := strings.Cut(rest, " val=")
		res.ai = aiStr == "true"
		res.level = level
		res.val = val
		return res
	}
	t.Fatalf("no RESULT line in helper output: %q", out.String())
	return res
}

func TestAILevel_resolution_table(t *testing.T) {
	cases := []struct {
		name       string
		env        string
		config     string
		flag       bool
		agent      bool
		wantAI     bool
		wantLevel  string
		wantStderr string
	}{
		{"env 1 without config file is masked", "1", "-", false, false, true, "masked", ""},
		{"env 1 with config lacking ai_mode is masked", "1", "", false, false, true, "masked", ""},
		{"env true enables masked by default", "true", "", false, false, true, "masked", ""},
		{"env strict forces strict", "strict", "", false, false, true, "strict", ""},
		{"env masked forces masked", "masked", "", false, false, true, "masked", ""},
		{"env masked overrides config strict", "masked", "ai_mode: strict\n", false, false, true, "masked", ""},
		{"env strict overrides config masked", "strict", "ai_mode: masked\n", false, false, true, "strict", ""},
		{"env 1 reads config strict", "1", "ai_mode: strict\n", false, false, true, "strict", ""},
		{"env 1 reads config masked", "1", "ai_mode: masked\n", false, false, true, "masked", ""},
		{"env true reads config strict", "true", "ai_mode: strict\n", false, false, true, "strict", ""},
		{"env 0 disables", "0", "ai_mode: strict\n", false, true, false, "", ""},
		{"env false disables even under agent", "false", "", false, true, false, "", ""},
		{"empty env without agent is off", "", "", false, false, false, "", ""},
		{"empty env with agent reads config strict", "", "ai_mode: strict\n", false, true, true, "strict", ""},
		{"empty env with agent and no config is masked", "", "-", false, true, true, "masked", ""},
		{"flag wins over env 0 and is masked by default", "0", "", true, false, true, "masked", ""},
		{"flag wins over env false and reads config strict", "false", "ai_mode: strict\n", true, false, true, "strict", ""},
		{"unknown env is fail-closed strict", "masekd", "", false, false, true, "strict", `ward: unknown WARD_AI_MODE value "masekd", using strict`},
		{"unknown env beats config masked", "masekd", "ai_mode: masked\n", false, false, true, "strict", `unknown WARD_AI_MODE value "masekd", using strict`},
		{"invalid config ai_mode is fail-closed strict", "1", "ai_mode: loose\n", false, false, true, "strict", `ward: unknown ai_mode "loose" in `},
		{"invalid config ai_mode under auto-detection", "", "ai_mode: loose\n", false, true, true, "strict", `unknown ai_mode "loose"`},
		{"env masked overrides invalid config", "masked", "ai_mode: loose\n", false, false, true, "masked", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// act
			got := runLevelHelper(t, tc.env, tc.config, tc.flag, tc.agent)

			// assert
			if got.ai != tc.wantAI {
				t.Errorf("IsAIMode() = %v, want %v", got.ai, tc.wantAI)
			}
			if tc.wantAI && got.level != tc.wantLevel {
				t.Errorf("AILevel() = %q, want %q", got.level, tc.wantLevel)
			}
			if tc.wantAI {
				want := "<sensitive>"
				if tc.wantLevel == "masked" {
					want = "tok_…a9f2 (51 chars)"
				}
				if got.val != want {
					t.Errorf("aiValue = %q, want %q", got.val, want)
				}
			}
			if tc.wantStderr != "" {
				if !strings.Contains(got.stderr, tc.wantStderr) {
					t.Errorf("stderr missing %q, got %q", tc.wantStderr, got.stderr)
				}
				if n := strings.Count(got.stderr, "unknown "); n != 1 {
					t.Errorf("expected the warning exactly once, got %d in %q", n, got.stderr)
				}
			} else if strings.Contains(got.stderr, "unknown ") {
				t.Errorf("unexpected warning on stderr: %q", got.stderr)
			}
		})
	}
}

func TestAILevel_constants(t *testing.T) {
	// assert
	if levelStrict != aiLevel("strict") || levelMasked != aiLevel("masked") {
		t.Errorf("unexpected level constants: %q %q", levelStrict, levelMasked)
	}
}

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

func TestAILevel_unreadable_config_is_strict_with_warning(t *testing.T) {
	// act
	got := runLevelHelper(t, "1", "ai_mode: [unclosed\n", false, false)

	// assert
	if !got.ai || got.level != "strict" {
		t.Errorf("ai=%v level=%q, want strict", got.ai, got.level)
	}
	if got.val != "<sensitive>" {
		t.Errorf("aiValue = %q, want <sensitive>", got.val)
	}
	if !strings.Contains(got.stderr, "cannot read") {
		t.Errorf("stderr missing %q, got %q", "cannot read", got.stderr)
	}
}

func TestAILevel_missing_config_is_masked_without_warning(t *testing.T) {
	// act
	got := runLevelHelper(t, "1", "-", false, false)

	// assert
	if got.level != "masked" {
		t.Errorf("level = %q, want masked", got.level)
	}
	if strings.Contains(got.stderr, "cannot read") {
		t.Errorf("unexpected warning: %q", got.stderr)
	}
}
