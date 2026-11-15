//go:build e2e

package subvault_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wardage "github.com/br4zz4/ward/internal/age"
	"github.com/br4zz4/ward/test/e2e/testutil"
)

var bin string

func TestMain(m *testing.M) {
	b, err := testutil.BuildBin()
	if err != nil {
		panic(err)
	}
	bin = b
	code := m.Run()
	os.Remove(b)
	os.Exit(code)
}

type vaultSpec struct {
	name    string
	folder  string
	keyFile string
	content string
}

type project struct {
	dir    string
	tokens map[string]string
}

func clearKeyEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"WARD_KEY",
		"WARD_KEY_MYFISH",
		"WARD_KEY_MYFISH_DEFAULT",
		"WARD_KEY_MYFISH_RESTRICTED",
		"WARD_KEY_MYFISH_OPEN",
	} {
		t.Setenv(k, "")
	}
}

func buildProject(t *testing.T, vaults ...vaultSpec) project {
	t.Helper()
	clearKeyEnv(t)
	dir := t.TempDir()
	wardDir := filepath.Join(dir, ".ward")
	if err := os.MkdirAll(wardDir, 0755); err != nil {
		t.Fatal(err)
	}
	p := project{dir: dir, tokens: map[string]string{}}
	var cfg strings.Builder
	cfg.WriteString("vaults:\n")
	for _, v := range vaults {
		folder := v.folder
		if folder == "" {
			folder = v.name
		}
		vaultRel := filepath.Join(".ward", "vaults", folder)
		vaultAbs := filepath.Join(dir, vaultRel)
		if err := os.MkdirAll(vaultAbs, 0755); err != nil {
			t.Fatal(err)
		}
		keyPath := filepath.Join(t.TempDir(), "key")
		if v.keyFile != "" {
			keyPath = filepath.Join(wardDir, v.keyFile)
		}
		if err := wardage.GenerateKey(keyPath); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(keyPath)
		if err != nil {
			t.Fatal(err)
		}
		p.tokens[v.name] = "ward-" + base64.URLEncoding.EncodeToString(raw)
		enc := wardage.AgeArmorDecryptor{KeyFile: keyPath}
		if err := enc.Encrypt(filepath.Join(vaultAbs, "secrets.ward"), []byte(v.content)); err != nil {
			t.Fatal(err)
		}
		cfg.WriteString("  - name: " + v.name + "\n    path: " + filepath.ToSlash(vaultRel) + "\n")
	}
	if err := os.WriteFile(filepath.Join(wardDir, "config.yaml"), []byte(cfg.String()), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

const defaultContent = `myfish::default:
  secrets:
    dev:
      DATABASE_URL: postgres://dev
    staging:
      DATABASE_URL: postgres://staging
`

const restrictedContent = `myfish::restricted:
  secrets:
    production:
      DATABASE_URL: postgres://prod
`

func twoVaults(t *testing.T) project {
	t.Helper()
	p := buildProject(t,
		vaultSpec{name: "myfish::default", content: defaultContent},
		vaultSpec{name: "myfish::restricted", content: restrictedContent},
	)
	t.Setenv("WARD_KEY_MYFISH_DEFAULT", p.tokens["myfish::default"])
	t.Setenv("WARD_KEY_MYFISH_RESTRICTED", p.tokens["myfish::restricted"])
	return p
}

func run(t *testing.T, p project, args ...string) (string, string, int) {
	t.Helper()
	out, errOut, code := testutil.Run(t, bin, p.dir, args...)
	return testutil.StripANSI(out), testutil.StripANSI(errOut), code
}

func secretsJSON(t *testing.T, p project, args ...string) map[string]string {
	t.Helper()
	full := append([]string{"secrets", "--json"}, args...)
	out, errOut, code := run(t, p, full...)
	if code != 0 {
		t.Fatalf("secrets --json exit %d\nstdout: %s\nstderr: %s", code, out, errOut)
	}
	got := map[string]string{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("parsing json %q: %v", out, err)
	}
	return got
}

func TestGet_restricted_subvault(t *testing.T) {
	// arrange
	p := twoVaults(t)
	// act
	out, errOut, code := run(t, p, "get", "myfish::restricted:secrets.production.DATABASE_URL")
	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "postgres://prod") {
		t.Errorf("got %q", out)
	}
}

func TestGet_default_alias_and_tree_shows_myfish(t *testing.T) {
	// arrange
	p := twoVaults(t)
	// act
	short, _, c1 := run(t, p, "get", "myfish:secrets.dev.DATABASE_URL")
	long, _, c2 := run(t, p, "get", "myfish::default:secrets.dev.DATABASE_URL")
	tree, _, c3 := run(t, p, "tree")
	// assert
	if c1 != 0 || c2 != 0 || c3 != 0 {
		t.Fatalf("exit codes %d %d %d", c1, c2, c3)
	}
	if strings.TrimSpace(short) != strings.TrimSpace(long) || !strings.Contains(short, "postgres://dev") {
		t.Errorf("short=%q long=%q", short, long)
	}
	hasRoot := false
	for _, line := range strings.Split(tree, "\n") {
		if strings.HasPrefix(line, "myfish::default") {
			t.Errorf("tree must show canonical root, got line %q", line)
		}
		if strings.HasPrefix(line, "myfish:") && !strings.HasPrefix(line, "myfish::") {
			hasRoot = true
		}
	}
	if !hasRoot {
		t.Errorf("tree missing myfish root: %q", tree)
	}
}

func TestGet_default_root_short_form(t *testing.T) {
	// arrange
	p := buildProject(t, vaultSpec{name: "myfish::default", content: strings.Replace(defaultContent, "myfish::default:", "myfish:", 1)})
	t.Setenv("WARD_KEY_MYFISH_DEFAULT", p.tokens["myfish::default"])
	// act
	out, errOut, code := run(t, p, "get", "myfish:secrets.dev.DATABASE_URL")
	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "postgres://dev") {
		t.Errorf("got %q", out)
	}
}

func TestSecrets_parent_qualifier_excludes_restricted(t *testing.T) {
	// arrange
	p := twoVaults(t)
	// act
	got := secretsJSON(t, p, "myfish:secrets.dev")
	// assert
	if got["DATABASE_URL"] != "postgres://dev" {
		t.Fatalf("expected default secrets, got %v", got)
	}
	for k, v := range got {
		if strings.Contains(v, "prod") {
			t.Errorf("restricted leaked: %s=%s", k, v)
		}
	}
}

func TestSecrets_parent_qualifier_prefixed_lists_only_default(t *testing.T) {
	// arrange
	p := twoVaults(t)
	// act
	got := secretsJSON(t, p, "--prefixed", "myfish:secrets")
	// assert
	if _, ok := got["myfish_secrets_dev_DATABASE_URL"]; !ok {
		t.Errorf("expected default key, got %v", got)
	}
	if _, ok := got["myfish_restricted_secrets_production_DATABASE_URL"]; ok {
		t.Errorf("restricted must not appear: %v", got)
	}
}

func TestSecrets_overlay_includes_both(t *testing.T) {
	// arrange
	p := twoVaults(t)
	// act
	got := secretsJSON(t, p, "--prefixed", "secrets")
	// assert
	if _, ok := got["myfish_secrets_dev_DATABASE_URL"]; !ok {
		t.Errorf("missing default: %v", got)
	}
	if _, ok := got["myfish_restricted_secrets_production_DATABASE_URL"]; !ok {
		t.Errorf("missing restricted: %v", got)
	}
}

func TestSecrets_prefixed_sanitizes_subvault_separator(t *testing.T) {
	// arrange
	p := twoVaults(t)
	// act
	got := secretsJSON(t, p, "--prefixed", "myfish::restricted:secrets")
	// assert
	if got["myfish_restricted_secrets_production_DATABASE_URL"] != "postgres://prod" {
		t.Errorf("got %v", got)
	}
}

func TestSecrets_missing_restricted_key_warns_and_default_loads(t *testing.T) {
	// arrange
	p := buildProject(t,
		vaultSpec{name: "myfish::default", content: defaultContent},
		vaultSpec{name: "myfish::restricted", content: restrictedContent},
	)
	t.Setenv("WARD_KEY_MYFISH", p.tokens["myfish::default"])
	// act
	out, errOut, code := run(t, p, "secrets", "--prefixed")
	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "postgres://dev") {
		t.Errorf("default must still load, got %q", out)
	}
	if strings.Contains(out, "postgres://prod") {
		t.Errorf("restricted must not load, got %q", out)
	}
	if !strings.Contains(errOut, "missing key for vault myfish::restricted") {
		t.Errorf("expected missing key warning, got %q", errOut)
	}
	if !strings.Contains(errOut, "WARD_KEY_MYFISH_RESTRICTED") {
		t.Errorf("expected env hint, got %q", errOut)
	}
}

func TestGet_default_env_alone_unlocks_default(t *testing.T) {
	// arrange
	p := buildProject(t, vaultSpec{name: "myfish::default", content: defaultContent})
	t.Setenv("WARD_KEY_MYFISH_DEFAULT", p.tokens["myfish::default"])
	// act
	out, errOut, code := run(t, p, "get", "myfish:secrets.dev.DATABASE_URL")
	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "postgres://dev") {
		t.Errorf("got %q", out)
	}
}

func TestGet_divergent_default_envs_fail_citing_both(t *testing.T) {
	// arrange
	p := buildProject(t, vaultSpec{name: "myfish::default", content: defaultContent})
	t.Setenv("WARD_KEY_MYFISH", p.tokens["myfish::default"])
	other := buildProject(t, vaultSpec{name: "x", content: "x:\n  a: b\n"})
	t.Setenv("WARD_KEY_MYFISH_DEFAULT", other.tokens["x"])
	t.Setenv("WARD_KEY_MYFISH", p.tokens["myfish::default"])
	// act
	_, errOut, code := run(t, p, "get", "myfish:secrets.dev.DATABASE_URL")
	// assert
	if code == 0 {
		t.Fatal("expected failure")
	}
	for _, want := range []string{"WARD_KEY_MYFISH", "WARD_KEY_MYFISH_DEFAULT"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr %q missing %q", errOut, want)
		}
	}
}

func TestGet_only_open_subvault_is_not_reached_by_parent(t *testing.T) {
	// arrange
	p := buildProject(t, vaultSpec{name: "myfish::open", content: "myfish::open:\n  secrets:\n    x: v\n"})
	t.Setenv("WARD_KEY_MYFISH_OPEN", p.tokens["myfish::open"])
	// act
	getOut, getErr, getCode := run(t, p, "get", "myfish:secrets.x")
	_, setErr, setCode := run(t, p, "set", "myfish:secrets.x", "v2")
	// assert
	if getCode == 0 {
		t.Fatalf("get must not reach myfish::open via myfish:, out=%q", getOut)
	}
	if !strings.Contains(getErr, `vault "myfish" not found`) {
		t.Errorf("expected not found on get, got %q", getErr)
	}
	if setCode == 0 {
		t.Fatal("set must fail for vault myfish")
	}
	if !strings.Contains(setErr, `vault "myfish" not found`) {
		t.Errorf("expected not found, got %q", setErr)
	}
}

func TestGet_open_not_decrypted_by_parent_env(t *testing.T) {
	// arrange
	p := buildProject(t, vaultSpec{name: "myfish::open", content: "myfish::open:\n  secrets:\n    x: v\n"})
	t.Setenv("WARD_KEY_MYFISH", p.tokens["myfish::open"])
	// act
	out, errOut, code := run(t, p, "get", "myfish::open:secrets.x")
	// assert
	if code == 0 {
		t.Fatalf("WARD_KEY_MYFISH must not decrypt myfish::open, got out=%q", out)
	}
	if !strings.Contains(errOut, "WARD_KEY_MYFISH_OPEN") {
		t.Errorf("expected hint citing WARD_KEY_MYFISH_OPEN, got %q", errOut)
	}
}

func TestSetUnset_in_restricted_subvault(t *testing.T) {
	// arrange
	p := twoVaults(t)
	// act
	_, setErr, setCode := run(t, p, "set", "myfish::restricted:secrets.production.API_KEY", "k-123")
	got, _, getCode := run(t, p, "get", "myfish::restricted:secrets.production.API_KEY")
	_, unsetErr, unsetCode := run(t, p, "unset", "myfish::restricted:secrets.production.API_KEY")
	_, _, afterCode := run(t, p, "get", "myfish::restricted:secrets.production.API_KEY")
	// assert
	if setCode != 0 {
		t.Fatalf("set exit %d: %s", setCode, setErr)
	}
	if getCode != 0 || !strings.Contains(got, "k-123") {
		t.Errorf("get after set: code=%d out=%q", getCode, got)
	}
	if unsetCode != 0 {
		t.Fatalf("unset exit %d: %s", unsetCode, unsetErr)
	}
	if afterCode == 0 {
		t.Error("expected key to be gone after unset")
	}
}

func TestSet_default_alias_preserves_declared_root(t *testing.T) {
	// arrange
	p := twoVaults(t)
	// act
	_, errOut, code := run(t, p, "set", "myfish:secrets.dev.API_KEY", "abc")
	out, _, getCode := run(t, p, "get", "myfish::default:secrets.dev.API_KEY")
	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %s", code, errOut)
	}
	if getCode != 0 || !strings.Contains(out, "abc") {
		t.Errorf("got code=%d out=%q", getCode, out)
	}
}

func TestFileAdd_into_restricted_subvault(t *testing.T) {
	// arrange
	p := twoVaults(t)
	src := filepath.Join(p.dir, "sa.json")
	if err := os.WriteFile(src, []byte(`{"type":"service_account"}`), 0600); err != nil {
		t.Fatal(err)
	}
	// act
	_, errOut, code := run(t, p, "file", "add", src, "myfish::restricted:certs")
	// assert
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	target := filepath.Join(p.dir, ".ward", "vaults", "myfish::restricted", "certs", "sa.json.ward")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("expected %s: %v", target, err)
	}
	if !strings.Contains(string(data), "BEGIN AGE ENCRYPTED FILE") {
		t.Errorf("file not encrypted: %q", data)
	}
}

func TestLegacy_project_layout_unchanged(t *testing.T) {
	// arrange
	p := buildProject(t, vaultSpec{
		name:    "myfish",
		keyFile: "myfish.key",
		content: "myfish:\n  secrets:\n    KEY: legacy-value\n",
	})
	// act
	got, errOut, code := run(t, p, "get", "myfish:secrets.KEY")
	list, _, listCode := run(t, p, "secrets")
	// assert
	if code != 0 {
		t.Fatalf("get exit %d: %s", code, errOut)
	}
	if !strings.Contains(got, "legacy-value") {
		t.Errorf("got %q", got)
	}
	if listCode != 0 || !strings.Contains(list, "legacy-value") {
		t.Errorf("secrets: code=%d out=%q", listCode, list)
	}
}

func TestConfig_invalid_names_fail(t *testing.T) {
	cases := map[string]string{
		"a::b::c": "invalid vault name",
		"::b":     "invalid vault name",
		"a::":     "invalid vault name",
		"a:b":     "invalid vault name",
	}
	for name, want := range cases {
		// arrange
		clearKeyEnv(t)
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, ".ward"), 0755)
		cfg := "vaults:\n  - name: \"" + name + "\"\n    path: .ward/vaults/x\n"
		os.WriteFile(filepath.Join(dir, ".ward", "config.yaml"), []byte(cfg), 0644)
		// act
		_, errOut, code := testutil.Run(t, bin, dir, "secrets")
		// assert
		if code == 0 {
			t.Errorf("%q: expected failure", name)
		}
		if !strings.Contains(testutil.StripANSI(errOut), want) {
			t.Errorf("%q: stderr %q missing %q", name, errOut, want)
		}
	}
}

func TestConfig_default_declared_twice_fails(t *testing.T) {
	// arrange
	clearKeyEnv(t)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".ward"), 0755)
	cfg := "vaults:\n  - name: myfish\n    path: .ward/vaults/a\n  - name: myfish::default\n    path: .ward/vaults/b\n"
	os.WriteFile(filepath.Join(dir, ".ward", "config.yaml"), []byte(cfg), 0644)
	// act
	_, errOut, code := testutil.Run(t, bin, dir, "secrets")
	// assert
	if code == 0 {
		t.Fatal("expected failure")
	}
	if !strings.Contains(errOut, "duplicate") {
		t.Errorf("got %q", errOut)
	}
}

func TestConfig_colliding_envs_fail(t *testing.T) {
	// arrange
	clearKeyEnv(t)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".ward"), 0755)
	cfg := "vaults:\n  - name: myfish-restricted\n    path: .ward/vaults/a\n  - name: myfish::restricted\n    path: .ward/vaults/b\n"
	os.WriteFile(filepath.Join(dir, ".ward", "config.yaml"), []byte(cfg), 0644)
	// act
	_, errOut, code := testutil.Run(t, bin, dir, "secrets")
	// assert
	if code == 0 {
		t.Fatal("expected failure")
	}
	for _, want := range []string{"WARD_KEY_MYFISH_RESTRICTED", "myfish-restricted", "myfish::restricted"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr %q missing %q", errOut, want)
		}
	}
}
