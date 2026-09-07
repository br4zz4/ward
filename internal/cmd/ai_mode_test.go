package cmd

import "testing"

func TestIsAIMode_defaults_false(t *testing.T) {
	// arrange: no flag, no env var
	t.Setenv("WARD_AI_MODE", "")
	SetAIMode(false)

	// act + assert
	if IsAIMode() {
		t.Errorf("expected AI mode OFF by default")
	}
}

func TestIsAIMode_env_var_1_enables(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "1")
	SetAIMode(false)

	// act + assert
	if !IsAIMode() {
		t.Errorf("expected WARD_AI_MODE=1 to enable AI mode")
	}
}

func TestIsAIMode_env_var_0_disables(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "0")
	SetAIMode(false)

	// act + assert
	if IsAIMode() {
		t.Errorf("expected WARD_AI_MODE=0 to keep AI mode OFF")
	}
}

func TestIsAIMode_flag_enables(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "")
	SetAIMode(true)

	// act + assert
	if !IsAIMode() {
		t.Errorf("expected --ai-mode flag to enable AI mode")
	}
}

func TestIsAIMode_flag_off_with_env_off(t *testing.T) {
	// arrange
	t.Setenv("WARD_AI_MODE", "")
	SetAIMode(false)

	// act + assert
	if IsAIMode() {
		t.Errorf("expected AI mode OFF when both flag and env are unset")
	}
}