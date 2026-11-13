package cmd

import "testing"

// clearAgentEnv clears every known agent marker so tests run in a "human"
// environment. Real agent markers (OPENCODE_*) leak into the test process when
// the test suite itself runs under an AI agent.
func clearAgentEnv(t *testing.T) {
	t.Helper()
	for _, k := range agentEnvKeys {
		t.Setenv(k, "")
	}
}

func TestIsAIMode_defaults_false(t *testing.T) {
	// arrange: no flag, no env var, no agent markers
	t.Setenv("WARD_AI_MODE", "")
	SetAIMode(false)
	clearAgentEnv(t)

	// act + assert
	if IsAIMode() {
		t.Errorf("expected AI mode OFF by default")
	}
}

func TestIsAIMode_env_var_1_enables(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "1")
	SetAIMode(false)
	clearAgentEnv(t)

	// act + assert
	if !IsAIMode() {
		t.Errorf("expected WARD_AI_MODE=1 to enable AI mode")
	}
}

func TestIsAIMode_env_var_0_disables(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "0")
	SetAIMode(false)
	clearAgentEnv(t)

	// act + assert
	if IsAIMode() {
		t.Errorf("expected WARD_AI_MODE=0 to keep AI mode OFF")
	}
}

func TestIsAIMode_flag_enables(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "")
	SetAIMode(true)
	clearAgentEnv(t)

	// act + assert
	if !IsAIMode() {
		t.Errorf("expected --ai-mode flag to enable AI mode")
	}
}

func TestIsAIMode_flag_off_with_env_off(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "")
	SetAIMode(false)
	clearAgentEnv(t)

	// act + assert
	if IsAIMode() {
		t.Errorf("expected AI mode OFF when both flag and env are unset")
	}
}

// --- agent auto-detection ----------------------------------------------------

func TestIsAIMode_detects_opencode(t *testing.T) {
	// arrange: no flag, no WARD_AI_MODE, but OPENCODE_* markers present
	t.Setenv("WARD_AI_MODE", "")
	SetAIMode(false)
	clearAgentEnv(t)
	t.Setenv("OPENCODE_PID", "12345")

	// act + assert
	if !IsAIMode() {
		t.Errorf("expected AI mode ON when running under OpenCode (OPENCODE_PID present)")
	}
}

func TestIsAIMode_detects_openclaw(t *testing.T) {
	t.Setenv("WARD_AI_MODE", "")
	SetAIMode(false)
	clearAgentEnv(t)
	t.Setenv("OPENCLAW_DEFAULT_MODEL", "deepseek/deepseek-v4-flash-0731")

	if !IsAIMode() {
		t.Errorf("expected AI mode ON when OPENCLAW_DEFAULT_MODEL present")
	}
}

func TestIsAIMode_detects_claude_code(t *testing.T) {
	t.Setenv("WARD_AI_MODE", "")
	SetAIMode(false)
	clearAgentEnv(t)
	t.Setenv("DEFAULT_CLAUDE_MODEL", "anthropic/claude-sonnet-4-6")

	if !IsAIMode() {
		t.Errorf("expected AI mode ON when DEFAULT_CLAUDE_MODEL present")
	}
}