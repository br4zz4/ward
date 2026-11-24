//go:build e2e

package ai_mode_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/br4zz4/ward/test/e2e/testutil"
)

const conflictValue1 = "SUPERSECRETVALUE1"
const conflictValue2 = "SUPERSECRETVALUE2"

func conflictProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	vault := filepath.Join(dir, ".ward", "vaults", "app")
	if err := os.MkdirAll(filepath.Join(vault, "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(dir, ".ward", "config.yaml"):    "vaults:\n  - name: app\n    path: .ward/vaults/app\n",
		filepath.Join(vault, "main.plain.ward"):       "app:\n  main:\n    db:\n      token: " + conflictValue1 + "\n    other: x\n",
		filepath.Join(vault, "main", "db.plain.ward"): "app:\n  main:\n    db:\n      token: " + conflictValue2 + "\n",
	}
	for p, body := range files {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func conflictCommands() map[string][]string {
	return map[string][]string{
		"get":        {"get", "app:main.db.token"},
		"check":      {"check", "app:main.db.token", "anything"},
		"set --from": {"set", "app:main.other", "--from", "app:main.db.token"},
		"tree":       {"tree"},
		"inspect":    {"inspect"},
	}
}

func TestConflictSnippet_never_leaks_values_in_ai_mode(t *testing.T) {
	for _, mode := range []string{"strict", "masked"} {
		for name, args := range conflictCommands() {
			t.Run(mode+"/"+name, func(t *testing.T) {
				// arrange
				dir := conflictProject(t)
				t.Setenv("WARD_AI_MODE", mode)

				// act
				out, stderr, _ := testutil.Run(t, bin, dir, args...)

				// assert
				got := testutil.StripANSI(out + stderr)
				for _, v := range []string{conflictValue1, conflictValue2} {
					if strings.Contains(got, v) {
						t.Errorf("value %q leaked: %q", v, got)
					}
				}
			})
		}
	}
}

func TestConflictSnippet_strict_shows_sensitive_placeholder(t *testing.T) {
	// arrange
	dir := conflictProject(t)
	t.Setenv("WARD_AI_MODE", "strict")

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "check", "app:main.db.token", "anything")

	// assert
	if code == 0 {
		t.Fatalf("expected failure on conflict, got exit 0 (%q)", out)
	}
	if !strings.Contains(testutil.StripANSI(stderr), "<sensitive>") {
		t.Errorf("expected <sensitive> in conflict output, got %q", stderr)
	}
}

func TestConflictSnippet_masked_shows_masked_form(t *testing.T) {
	// arrange
	dir := conflictProject(t)
	t.Setenv("WARD_AI_MODE", "masked")

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "check", "app:main.db.token", "anything")

	// assert
	if code == 0 {
		t.Fatal("expected failure on conflict")
	}
	if !strings.Contains(testutil.StripANSI(stderr), "(17 chars)") {
		t.Errorf("expected masked form in conflict output, got %q", stderr)
	}
}

func TestConflictSnippet_outside_ai_mode_still_shows_snippet(t *testing.T) {
	// arrange
	dir := conflictProject(t)
	t.Setenv("WARD_AI_MODE", "0")

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "check", "app:main.db.token", "anything")

	// assert
	if code == 0 {
		t.Fatal("expected failure on conflict")
	}
	got := testutil.StripANSI(stderr)
	if !strings.Contains(got, "token: "+conflictValue1) || !strings.Contains(got, "token: "+conflictValue2) {
		t.Errorf("expected full snippets outside AI mode, got %q", got)
	}
}

func TestSecretsJSON_ai_mode_exits_3(t *testing.T) {
	for _, mode := range []string{"1", "strict", "masked"} {
		t.Run(mode, func(t *testing.T) {
			// arrange
			t.Setenv("WARD_AI_MODE", mode)

			// act
			out, _, code := testutil.Run(t, bin, treeFixture(), "secrets", "--json")

			// assert
			if code != 3 {
				t.Errorf("expected exit 3, got %d", code)
			}
			if out != "" {
				t.Errorf("expected empty stdout, got %q", out)
			}
		})
	}
}

type conflictCase struct {
	name       string
	files      map[string]string
	fileSecret string
	secret     string
	scope      string
	leaks      []string
	rawShown   []string
}

func (c conflictCase) project(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if testutil.RunCmd(t, "cp", "-r", testutil.FixtureDir("file", "basic")+"/.", dir) != 0 {
		t.Fatal("copy fixture failed")
	}
	vault := filepath.Join(dir, ".ward", "vaults", "app")
	if err := os.MkdirAll(filepath.Join(vault, "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, body := range c.files {
		if err := os.WriteFile(filepath.Join(vault, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if c.fileSecret != "" {
		src := filepath.Join(t.TempDir(), "cred.json")
		if err := os.WriteFile(src, []byte(c.secret), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, stderr, code := testutil.Run(t, bin, dir, "file", "add", src, "app"); code != 0 {
			t.Fatalf("file add exit %d: %s", code, stderr)
		}
	}
	return dir
}

func (c conflictCase) commands() map[string][]string {
	return map[string][]string{
		"get":        {"get", c.scope},
		"check":      {"check", c.scope, "anything"},
		"set --from": {"set", "app:main.other", "--from", c.scope},
		"tree":       {"tree"},
		"inspect":    {"inspect"},
	}
}

func conflictCases() []conflictCase {
	return []conflictCase{
		{
			name:       "file secret raw content",
			fileSecret: "cred.json",
			secret:     "token=FSECRETRAW99; note: x",
			files: map[string]string{
				"main.plain.ward":      "app:\n  main:\n    other: x\n",
				"cred_json.plain.ward": "app:\n  cred_json: PLAINVALUE9\n",
			},
			scope:    "app:cred_json",
			leaks:    []string{"FSECRETRAW99", "token=FSECRETRAW99", "note: x", "PLAINVALUE9"},
			rawShown: []string{"token=FSECRETRAW99"},
		},
		{
			name:       "file secret one-line json",
			fileSecret: "cred.json",
			secret:     `{"k":"JSONSECRET77","z":1}`,
			files: map[string]string{
				"main.plain.ward":      "app:\n  main:\n    other: x\n",
				"cred_json.plain.ward": "app:\n  cred_json: PLAINVALUE9\n",
			},
			scope:    "app:cred_json",
			leaks:    []string{"JSONSECRET77", `{"k":"JSONSECRET77","z":1}`, "PLAINVALUE9"},
			rawShown: []string{"JSONSECRET77"},
		},
		{
			name: "short value with yaml comment",
			files: map[string]string{
				"main.plain.ward":     "app:\n  main:\n    other: x\n    pin: 1234 # code\n",
				"main/pin.plain.ward": "app:\n  main:\n    pin: 5678\n",
			},
			scope:    "app:main.pin",
			leaks:    []string{"1234", "5678", "# code"},
			rawShown: []string{"pin: 1234 # code", "pin: 5678"},
		},
		{
			name: "map in conflict with leaf",
			files: map[string]string{
				"main.plain.ward":    "app:\n  main:\n    other: x\n    db: LEAFSECRET55\n",
				"main/db.plain.ward": "app:\n  main:\n    db:\n      token: MAPSECRET66\n      user: MAPUSER88\n",
			},
			scope:    "app:main.db",
			leaks:    []string{"LEAFSECRET55", "MAPSECRET66", "MAPUSER88"},
			rawShown: []string{"LEAFSECRET55"},
		},
	}
}

func TestConflictSnippet_structured_cases_never_leak_in_ai_mode(t *testing.T) {
	for _, tc := range conflictCases() {
		for _, mode := range []string{"strict", "masked"} {
			for name, args := range tc.commands() {
				t.Run(tc.name+"/"+mode+"/"+name, func(t *testing.T) {
					// arrange
					t.Setenv("WARD_AI_MODE", "0")
					dir := tc.project(t)
					t.Setenv("WARD_AI_MODE", mode)

					// act
					out, stderr, _ := testutil.Run(t, bin, dir, args...)

					// assert
					got := testutil.StripANSI(out + stderr)
					for _, v := range tc.leaks {
						if strings.Contains(got, v) {
							t.Errorf("%q leaked: %q", v, got)
						}
					}
				})
			}
		}
	}
}

func TestConflictSnippet_structured_cases_stay_raw_outside_ai_mode(t *testing.T) {
	for _, tc := range conflictCases() {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			t.Setenv("WARD_AI_MODE", "0")
			dir := tc.project(t)

			// act
			out, stderr, code := testutil.Run(t, bin, dir, "check", tc.scope, "anything")

			// assert
			if code == 0 {
				t.Fatalf("expected conflict failure, got %q", out)
			}
			got := testutil.StripANSI(stderr)
			for _, v := range tc.rawShown {
				if !strings.Contains(got, v) {
					t.Errorf("expected raw %q outside AI mode, got %q", v, got)
				}
			}
		})
	}
}

func TestConflictSnippet_masked_short_value_length_is_the_parsed_value(t *testing.T) {
	// arrange
	tc := conflictCases()[2]
	t.Setenv("WARD_AI_MODE", "0")
	dir := tc.project(t)
	t.Setenv("WARD_AI_MODE", "masked")

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "check", tc.scope, "anything")

	// assert
	if code == 0 {
		t.Fatal("expected conflict failure")
	}
	got := testutil.StripANSI(stderr)
	if strings.Count(got, "pin: … (4 chars)") != 2 {
		t.Errorf("expected both origins as 'pin: … (4 chars)', got %q", got)
	}
}
