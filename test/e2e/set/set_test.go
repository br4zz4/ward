//go:build e2e

package set_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

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

func fix(name string) string { return testutil.FixtureDir("set", name) }

// clearKeyEnv unsets every env var that could unlock the locked-vault fixture,
// so a developer's environment cannot mask a failure. The child process
// inherits the parent's environment.
func clearKeyEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"WARD_KEY",                // global token, checked first
		"WARD_KEY_LOCKED",         // derived from vault name "locked"
		"WARD_KEY_APP",            // derived from vault name "app"
		"WARD_KEY_LOCKED_FIXTURE", // declared via key_env
	} {
		t.Setenv(name, "")
	}
}

// copyFixture copies a fixture directory to dst so tests can mutate it.
func copyFixture(t *testing.T, src, dst string) {
	t.Helper()
	if testutil.RunCmd(t, "cp", "-r", src+"/.", dst) != 0 {
		t.Fatalf("copy fixture failed")
	}
}

func TestSet_updates_existing_key(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	_, _, code := testutil.Run(t, bin, dir, "set", "app:main.token", "updated")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d", code)
	}
	out, _, _ := testutil.Run(t, bin, dir, "get", "app:main.token")
	if !testutil.Contains(out, "updated") {
		t.Errorf("expected updated, got: %q", out)
	}
}

func TestSet_updates_with_vault_secret_flags(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	_, _, code := testutil.Run(t, bin, dir, "set", "--vault", "app", "--secret", "main.token", "v")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d", code)
	}
	out, _, _ := testutil.Run(t, bin, dir, "get", "app:main.token")
	if !testutil.Contains(out, "v") {
		t.Errorf("expected v, got: %q", out)
	}
}

func TestSet_updates_with_scope_flag(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	_, _, code := testutil.Run(t, bin, dir, "set", "-s", "app:main.token", "v")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d", code)
	}
	out, _, _ := testutil.Run(t, bin, dir, "get", "app:main.token")
	if !testutil.Contains(out, "v") {
		t.Errorf("expected v, got: %q", out)
	}
}

func TestSet_creates_new_file_with_notice(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "app:prod.apikey", "xyz")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d", code)
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "a new file was created") {
		t.Errorf("expected new-file notice, got: %q", stderr)
	}
	out, _, _ := testutil.Run(t, bin, dir, "get", "app:prod.apikey")
	if !testutil.Contains(out, "xyz") {
		t.Errorf("expected xyz, got: %q", out)
	}
}

func TestSet_shallow_path_fails(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "app:token", "x")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for shallow path")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "too shallow") {
		t.Errorf("expected too-shallow error, got: %q", stderr)
	}
}

func TestSet_unknown_vault_fails(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "unknown:main.token", "x")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown vault")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "not found") {
		t.Errorf("expected vault-not-found error, got: %q", stderr)
	}
}

func TestSet_new_key_preserves_sibling_keys_in_same_file(t *testing.T) {
	// arrange: basic fixture has app.main.{name, token}
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)

	// act: add a brand-new key to the same file
	_, _, code := testutil.Run(t, bin, dir, "set", "app:main.password", "s3cr3t")

	// assert: the new key was written
	if code != 0 {
		t.Fatalf("set exit %d", code)
	}
	out, _, _ := testutil.Run(t, bin, dir, "get", "app:main.password")
	if !testutil.Contains(out, "s3cr3t") {
		t.Errorf("expected password=s3cr3t, got: %q", out)
	}

	// assert: pre-existing sibling keys must still be present
	out, _, _ = testutil.Run(t, bin, dir, "get", "app:main.token")
	if !testutil.Contains(out, "original") {
		t.Errorf("expected token=original to be preserved, got: %q", out)
	}
	out, _, _ = testutil.Run(t, bin, dir, "get", "app:main.name")
	if !testutil.Contains(out, "my-service") {
		t.Errorf("expected name=my-service to be preserved, got: %q", out)
	}
}

func TestSet_group_path_fails_with_children_listed(t *testing.T) {
	// arrange: group fixture has app.main.db.{host, port} — app.main.db is a group
	dir := t.TempDir()
	copyFixture(t, fix("group"), dir)

	// act: try to set a value at a group path (3 segments, passes requireLeafDepth)
	_, stderr, code := testutil.Run(t, bin, dir, "set", "app:main.db", "some-value")

	// assert: must fail
	if code == 0 {
		t.Fatal("expected non-zero exit when setting a group path")
	}
	stripped := testutil.StripANSI(stderr)
	if !testutil.Contains(stripped, "group") {
		t.Errorf("expected error to mention 'group', got: %q", stripped)
	}
	if !testutil.Contains(stripped, "host") || !testutil.Contains(stripped, "port") {
		t.Errorf("expected error to list child keys (host, port), got: %q", stripped)
	}
}

func TestSet_type2_collision_writes_and_warns(t *testing.T) {
	// arrange
	dir := t.TempDir()
	copyFixture(t, fix("collision"), dir)

	// act: vb.prod.token collides with va.staging.token on env var TOKEN
	_, stderr, code := testutil.Run(t, bin, dir, "set", "vb:prod.token", "from-vb")

	// assert: write succeeds AND a warning is printed
	if code != 0 {
		t.Fatalf("set exit %d — expected success with warning", code)
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "collides") {
		t.Errorf("expected collision warning, got: %q", stderr)
	}
	out, _, _ := testutil.Run(t, bin, dir, "get", "vb:prod.token")
	if !testutil.Contains(out, "from-vb") {
		t.Errorf("expected value written despite collision, got: %q", out)
	}
}

// An unqualified write must not be resolved from a partial view: a vault skipped
// for lack of a key may define the same path, so the target is ambiguous.
func TestSet_unqualified_refuses_when_a_vault_is_locked(t *testing.T) {
	// arrange
	clearKeyEnv(t)
	dir := t.TempDir()
	copyFixture(t, fix("locked-vault"), dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "main.db.host", "changed")

	// assert
	if code == 0 {
		t.Fatal("expected unqualified set to fail while a vault is locked")
	}
	stripped := testutil.StripANSI(stderr)
	if !testutil.Contains(stripped, "locked") {
		t.Errorf("expected the error to name the skipped vault, got: %q", stripped)
	}
	// the readable vault must be untouched
	out, _, _ := testutil.Run(t, bin, dir, "get", "app:main.db.host")
	if !testutil.Contains(out, "localhost") {
		t.Errorf("expected app.main.db.host unchanged, got: %q", out)
	}
}

// A qualified write still works while another vault is locked: the target is
// explicit, so no guessing is involved.
func TestSet_qualified_still_works_when_another_vault_is_locked(t *testing.T) {
	// arrange
	clearKeyEnv(t)
	dir := t.TempDir()
	copyFixture(t, fix("locked-vault"), dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "app:main.db.host", "changed")

	// assert
	if code != 0 {
		t.Fatalf("qualified set should succeed, got %d — %q", code, testutil.StripANSI(stderr))
	}
	out, _, _ := testutil.Run(t, bin, dir, "get", "app:main.db.host")
	if !testutil.Contains(out, "changed") {
		t.Errorf("expected value written, got: %q", out)
	}
}

func newProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	copyFixture(t, fix("basic"), dir)
	return dir
}

func writeValue(t *testing.T, content []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "value")
	if err := os.WriteFile(p, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func snapshot(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		out[p] = b
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func assertUntouched(t *testing.T, before, after map[string][]byte) {
	t.Helper()
	if !reflect.DeepEqual(before, after) {
		t.Errorf("project files were created or altered by a failed set")
	}
}

func getValue(t *testing.T, dir, scope string) string {
	t.Helper()
	out, stderr, code := testutil.Run(t, bin, dir, "get", scope)
	if code != 0 {
		t.Fatalf("get %s exit %d: %q", scope, code, stderr)
	}
	return out
}

func TestSet_value_from_file(t *testing.T) {
	// arrange
	dir := newProject(t)
	p := writeValue(t, []byte("projects/1/locations/global/x\n"))

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "app:ci.WIF", "@"+p)

	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %q", code, stderr)
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "(from "+p+")") {
		t.Errorf("expected (from %s) in stderr, got: %q", p, stderr)
	}
	if got := getValue(t, dir, "app:ci.WIF"); got != "projects/1/locations/global/x\n" {
		t.Errorf("expected value without trailing newline, got: %q", got)
	}
}

func TestSet_value_from_multiline_file_preserves_inner_newlines(t *testing.T) {
	// arrange
	dir := newProject(t)
	json3 := "{\n  \"type\": \"service_account\",\n  \"id\": \"1\"\n}"
	p := writeValue(t, []byte(json3+"\n"))

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "app:ci.SA_JSON", "@"+p)

	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %q", code, stderr)
	}
	if got := getValue(t, dir, "app:ci.SA_JSON"); got != json3+"\n" {
		t.Errorf("expected multiline value preserved, got: %q", got)
	}
}

func TestSet_value_from_file_removes_trailing_crlf(t *testing.T) {
	// arrange
	dir := newProject(t)
	p := writeValue(t, []byte("abc\r\n"))

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "app:ci.k", "@"+p)

	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %q", code, stderr)
	}
	if got := getValue(t, dir, "app:ci.k"); got != "abc\n" {
		t.Errorf("expected abc without CR, got: %q", got)
	}
}

func TestSet_missing_file_fails_and_writes_nothing(t *testing.T) {
	// arrange
	dir := newProject(t)
	missing := filepath.Join(t.TempDir(), "nao-existe")
	before := snapshot(t, dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "app:ci.k", "@"+missing)

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for missing file")
	}
	stripped := testutil.StripANSI(stderr)
	if !testutil.Contains(stripped, missing) || !testutil.Contains(stripped, "--literal") {
		t.Errorf("expected stderr with path and --literal, got: %q", stripped)
	}
	assertUntouched(t, before, snapshot(t, dir))
}

func TestSet_empty_file_fails_and_writes_nothing(t *testing.T) {
	// arrange
	dir := newProject(t)
	p := writeValue(t, []byte("\n"))
	before := snapshot(t, dir)

	// act
	_, _, code := testutil.Run(t, bin, dir, "set", "app:ci.k", "@"+p)

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for empty value")
	}
	assertUntouched(t, before, snapshot(t, dir))
}

func TestSet_non_utf8_file_fails_and_writes_nothing(t *testing.T) {
	// arrange
	dir := newProject(t)
	p := writeValue(t, []byte{0xff, 0xfe})
	before := snapshot(t, dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "app:ci.k", "@"+p)

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for non UTF-8 content")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "UTF-8") {
		t.Errorf("expected stderr to mention UTF-8, got: %q", stderr)
	}
	assertUntouched(t, before, snapshot(t, dir))
}

func TestSet_file_above_one_mib_fails_and_writes_nothing(t *testing.T) {
	// arrange
	dir := newProject(t)
	p := writeValue(t, bytes.Repeat([]byte("a"), 1048577))
	before := snapshot(t, dir)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "app:ci.k", "@"+p)

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit above 1 MiB")
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "limit") {
		t.Errorf("expected stderr to mention the limit, got: %q", stderr)
	}
	assertUntouched(t, before, snapshot(t, dir))
}

func TestSet_relative_file_resolves_against_invocation_dir_with_dir_flag(t *testing.T) {
	// arrange
	proj := newProject(t)
	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "v"), []byte("from-work\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// act
	_, stderr, code := testutil.Run(t, bin, work, "--dir", proj, "set", "app:ci.k", "@v")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %q", code, stderr)
	}
	if got := getValue(t, proj, "app:ci.k"); got != "from-work\n" {
		t.Errorf("expected content of the file in the invocation dir, got: %q", got)
	}
}

func TestSet_double_at_escape_stores_single_at(t *testing.T) {
	// arrange
	dir := newProject(t)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "app:ci.k", "@@org/x")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %q", code, stderr)
	}
	if got := getValue(t, dir, "app:ci.k"); got != "@org/x\n" {
		t.Errorf("expected @org/x, got: %q", got)
	}
	if testutil.Contains(testutil.StripANSI(stderr), "(from") {
		t.Errorf("expected no origin for a literal, got: %q", stderr)
	}
}

func TestSet_literal_flag_stores_at_path_without_reading(t *testing.T) {
	// arrange
	dir := newProject(t)
	p := writeValue(t, []byte("file-content\n"))

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "--literal", "app:ci.k", "@"+p)

	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %q", code, stderr)
	}
	if got := getValue(t, dir, "app:ci.k"); got != "@"+p+"\n" {
		t.Errorf("expected literal @path, got: %q", got)
	}
	if testutil.Contains(testutil.StripANSI(stderr), "(from") {
		t.Errorf("expected no origin for a literal, got: %q", stderr)
	}
}

func TestSet_literal_flag_stores_dash(t *testing.T) {
	// arrange
	dir := newProject(t)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "--literal", "app:ci.k", "-")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %q", code, stderr)
	}
	if got := getValue(t, dir, "app:ci.k"); got != "-\n" {
		t.Errorf("expected literal dash, got: %q", got)
	}
}

func TestSet_literal_flag_stores_double_at_as_is(t *testing.T) {
	// arrange
	dir := newProject(t)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "--literal", "app:ci.k", "@@x")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %q", code, stderr)
	}
	if got := getValue(t, dir, "app:ci.k"); got != "@@x\n" {
		t.Errorf("expected @@x, got: %q", got)
	}
}

func TestSet_literal_flag_with_vault_and_secret_flags(t *testing.T) {
	// arrange
	dir := newProject(t)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "--vault", "app", "--secret", "ci.k", "--literal", "@x")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %q", code, stderr)
	}
	if got := getValue(t, dir, "app:ci.k"); got != "@x\n" {
		t.Errorf("expected @x, got: %q", got)
	}
}

func TestSet_value_from_stdin_pipe(t *testing.T) {
	// arrange
	dir := newProject(t)

	// act
	_, stderr, code := testutil.RunWithStdinFull(t, bin, dir, "s3cr3t\n", "set", "app:ci.k", "-")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %q", code, stderr)
	}
	if !testutil.Contains(testutil.StripANSI(stderr), "(from stdin)") {
		t.Errorf("expected (from stdin), got: %q", stderr)
	}
	if got := getValue(t, dir, "app:ci.k"); got != "s3cr3t\n" {
		t.Errorf("expected s3cr3t, got: %q", got)
	}
}

func TestSet_empty_stdin_fails_and_writes_nothing(t *testing.T) {
	// arrange
	dir := newProject(t)
	before := snapshot(t, dir)

	// act
	_, _, code := testutil.RunWithStdinFull(t, bin, dir, "", "set", "app:ci.k", "-")

	// assert
	if code == 0 {
		t.Fatal("expected non-zero exit for empty stdin")
	}
	assertUntouched(t, before, snapshot(t, dir))
}

func TestSet_plain_literal_has_no_origin_in_stderr(t *testing.T) {
	// arrange
	dir := newProject(t)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "set", "app:ci.k", "postgres://x")

	// assert
	if code != 0 {
		t.Fatalf("set exit %d: %q", code, stderr)
	}
	if testutil.Contains(testutil.StripANSI(stderr), "(from") {
		t.Errorf("expected no (from in stderr, got: %q", stderr)
	}
	if got := getValue(t, dir, "app:ci.k"); got != "postgres://x\n" {
		t.Errorf("expected postgres://x, got: %q", got)
	}
}
