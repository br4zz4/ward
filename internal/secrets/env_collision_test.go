package secrets

import (
	"strings"
	"testing"
)

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if s[i] == 'm' {
				inEsc = false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func TestEnvConflictError_envs_shows_envs_examples(t *testing.T) {
	// arrange
	err := &EnvConflictError{
		Cmd: "envs",
		Conflicts: []EnvConflict{
			{EnvKey: "TOKEN", DotPaths: [2]string{"app.staging.token", "app.production.token"}},
		},
	}

	// act
	msg := stripANSI(err.Error())

	// assert
	if !strings.Contains(msg, "ward envs app.staging") {
		t.Errorf("expected 'ward envs' scoped example, got:\n%s", msg)
	}
	if strings.Contains(msg, "ward exec") {
		t.Errorf("did not expect 'ward exec' in envs message, got:\n%s", msg)
	}
}

func TestEnvConflictError_exec_shows_exec_examples(t *testing.T) {
	// arrange
	err := &EnvConflictError{
		Cmd: "exec",
		Conflicts: []EnvConflict{
			{EnvKey: "TOKEN", DotPaths: [2]string{"app.staging.token", "app.production.token"}},
		},
	}

	// act
	msg := stripANSI(err.Error())

	// assert
	if !strings.Contains(msg, "ward exec app.staging -- <cmd>") {
		t.Errorf("expected 'ward exec ... -- <cmd>' example, got:\n%s", msg)
	}
}

func TestEnvConflictError_shows_prefixed_preview(t *testing.T) {
	// arrange
	err := &EnvConflictError{
		Cmd: "envs",
		Conflicts: []EnvConflict{
			{EnvKey: "TOKEN", DotPaths: [2]string{"app.staging.token", "app.production.token"}},
		},
	}

	// act
	msg := stripANSI(err.Error())

	// assert: shows the full-path names --prefixed would produce
	if !strings.Contains(msg, "app_staging_token") || !strings.Contains(msg, "app_production_token") {
		t.Errorf("expected prefixed name preview, got:\n%s", msg)
	}
}

func TestPrefixedName_joins_full_path(t *testing.T) {
	// arrange / act / assert
	if got := prefixedName("app.staging.db-url"); got != "app_staging_db_url" {
		t.Errorf("expected app_staging_db_url, got %q", got)
	}
}
