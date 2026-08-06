package ward

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/sops"
)

// A vault with an encrypted file but no key is skipped with a warning;
// a plain file in another vault still loads.
func TestLoad_skips_keyless_encrypted_and_warns(t *testing.T) {
	dir := t.TempDir()
	vA := filepath.Join(dir, "a")
	vB := filepath.Join(dir, "b")
	os.MkdirAll(vA, 0755)
	os.MkdirAll(vB, 0755)
	// encrypted file in vault a (armored, no key)
	os.WriteFile(filepath.Join(vA, "main.ward"),
		[]byte("-----BEGIN AGE ENCRYPTED FILE-----\nxxx\n"), 0644)
	// plain file in vault b
	os.WriteFile(filepath.Join(vB, "conf.plain.ward"),
		[]byte("b:\n  ok: \"1\"\n"), 0644)

	cfg := &config.Config{
		Encryption: config.Encryption{Engine: "age+armor"},
		Vaults: []config.Source{
			{Name: "a", Path: vA},
			{Name: "b", Path: vB},
		},
	}
	// no vault decryptors, global fallback requires key
	eng := NewEngine(cfg, sops.RequireKeyDecryptor{})
	res, err := eng.MergeForView()
	if err != nil {
		t.Fatalf("expected no error (b is loadable), got %v", err)
	}
	if res.Tree["b"] == nil {
		t.Error("expected vault b to be present in tree")
	}
	warns := eng.Warnings()
	if len(warns) != 1 || !strings.Contains(warns[0], "a") {
		t.Errorf("expected one warning mentioning vault a, got %v", warns)
	}
}

// No key anywhere and only encrypted files → error.
func TestLoad_all_keyless_encrypted_errors(t *testing.T) {
	dir := t.TempDir()
	vA := filepath.Join(dir, "a")
	os.MkdirAll(vA, 0755)
	os.WriteFile(filepath.Join(vA, "main.ward"),
		[]byte("-----BEGIN AGE ENCRYPTED FILE-----\nxxx\n"), 0644)

	cfg := &config.Config{
		Encryption: config.Encryption{Engine: "age+armor"},
		Vaults:     []config.Source{{Name: "a", Path: vA}},
	}
	eng := NewEngine(cfg, sops.RequireKeyDecryptor{})
	if _, err := eng.MergeForView(); err == nil {
		t.Fatal("expected error when nothing is decryptable")
	}
}

// A vault whose declared key is unavailable is skipped with a warning that
// carries the hint, while the other vault still loads.
func TestLoad_missing_vault_key_warns_with_hint(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vA := filepath.Join(dir, "a")
	vB := filepath.Join(dir, "b")
	os.MkdirAll(vA, 0755)
	os.MkdirAll(vB, 0755)
	os.WriteFile(filepath.Join(vA, "main.ward"),
		[]byte("-----BEGIN AGE ENCRYPTED FILE-----\nxxx\n"), 0644)
	os.WriteFile(filepath.Join(vB, "conf.plain.ward"),
		[]byte("b:\n  ok: \"1\"\n"), 0644)

	cfg := &config.Config{
		Encryption: config.Encryption{Engine: "age+armor"},
		Vaults: []config.Source{
			{Name: "a", Path: vA},
			{Name: "b", Path: vB},
		},
	}
	eng := NewEngineWithVaultDecryptors(cfg, sops.RequireKeyDecryptor{}, map[string]sops.Decryptor{
		"a": sops.MissingKeyDecryptor{Hint: "set WARD_KEY_A to the contents of your age key"},
	})

	// act
	res, err := eng.MergeForView()

	// assert
	if err != nil {
		t.Fatalf("expected no error (b is loadable), got %v", err)
	}
	if res.Tree["b"] == nil {
		t.Error("expected vault b to be present in tree")
	}
	warns := eng.Warnings()
	if len(warns) != 1 || !strings.Contains(warns[0], "WARD_KEY_A") {
		t.Errorf("expected warning carrying the hint, got %v", warns)
	}
}

// Writing to a vault whose key is merely unavailable must fail loudly instead of
// replacing the encrypted file with plaintext.
func TestEncrypt_refuses_when_vault_key_is_missing(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vA := filepath.Join(dir, "a")
	os.MkdirAll(vA, 0755)
	path := filepath.Join(vA, "main.ward")
	ciphertext := []byte("-----BEGIN AGE ENCRYPTED FILE-----\nxxx\n")
	os.WriteFile(path, ciphertext, 0644)

	cfg := &config.Config{
		Encryption: config.Encryption{Engine: "age+armor"},
		Vaults:     []config.Source{{Name: "a", Path: vA}},
	}
	eng := NewEngineWithVaultDecryptors(cfg, sops.RequireKeyDecryptor{}, map[string]sops.Decryptor{
		"a": sops.MissingKeyDecryptor{Hint: "set WARD_KEY_A to the contents of your age key"},
	})

	// act
	err := eng.Encrypt(path, []byte("a:\n  leaked: \"1\"\n"))

	// assert
	if err == nil {
		t.Fatal("expected Encrypt to refuse without a key")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(ciphertext) {
		t.Errorf("encrypted file was overwritten: %q", got)
	}
}

// Writing plaintext over an existing ciphertext must be refused even when no key
// is declared anywhere — the fallback plain write is only for plain projects.
func TestEncrypt_refuses_to_clobber_ciphertext_without_any_key(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vA := filepath.Join(dir, "a")
	os.MkdirAll(vA, 0755)
	path := filepath.Join(vA, "main.ward")
	ciphertext := []byte("-----BEGIN AGE ENCRYPTED FILE-----\nxxx\n")
	os.WriteFile(path, ciphertext, 0644)

	cfg := &config.Config{
		Encryption: config.Encryption{Engine: "age+armor"},
		Vaults:     []config.Source{{Name: "a", Path: vA}},
	}
	// no vault decryptors at all — the global fallback merely requires a key
	eng := NewEngine(cfg, sops.RequireKeyDecryptor{})

	// act
	err := eng.Encrypt(path, []byte("a:\n  leaked: \"1\"\n"))

	// assert
	if err == nil {
		t.Fatal("expected Encrypt to refuse overwriting ciphertext")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(ciphertext) {
		t.Errorf("encrypted file was overwritten: %q", got)
	}
}

// A project with no encryption at all still writes plainly — the guard above
// must not break the documented plain-project fallback.
func TestEncrypt_still_writes_plain_when_no_encryption_configured(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vA := filepath.Join(dir, "a")
	os.MkdirAll(vA, 0755)
	path := filepath.Join(vA, "main.ward")
	os.WriteFile(path, []byte("a:\n  old: \"1\"\n"), 0644)

	cfg := &config.Config{Vaults: []config.Source{{Name: "a", Path: vA}}}
	eng := NewEngine(cfg, sops.RequireKeyDecryptor{})

	// act
	want := "a:\n  new: \"2\"\n"
	if err := eng.Encrypt(path, []byte(want)); err != nil {
		t.Fatalf("expected plain write to succeed, got %v", err)
	}

	// assert
	got, _ := os.ReadFile(path)
	if string(got) != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// Every skipped vault must appear in the fatal guidance, including one that
// declared no key source and therefore carries no hint.
func TestLoad_fatal_summary_covers_vaults_without_a_hint(t *testing.T) {
	// arrange: vault a declares a key (hinted), vault b declares none (unhinted)
	dir := t.TempDir()
	vA := filepath.Join(dir, "a")
	vB := filepath.Join(dir, "b")
	os.MkdirAll(vA, 0755)
	os.MkdirAll(vB, 0755)
	armor := []byte("-----BEGIN AGE ENCRYPTED FILE-----\nxxx\n")
	os.WriteFile(filepath.Join(vA, "main.ward"), armor, 0644)
	os.WriteFile(filepath.Join(vB, "main.ward"), armor, 0644)

	cfg := &config.Config{
		Encryption: config.Encryption{Engine: "age+armor"},
		Vaults: []config.Source{
			{Name: "a", Path: vA},
			{Name: "b", Path: vB},
		},
	}
	eng := NewEngineWithVaultDecryptors(cfg, sops.RequireKeyDecryptor{}, map[string]sops.Decryptor{
		"a": sops.MissingKeyDecryptor{Hint: "set WARD_KEY_A to the contents of your age key"},
	})

	// act
	_, err := eng.MergeForView()

	// assert
	if err == nil {
		t.Fatal("expected error when nothing is decryptable")
	}
	if !strings.Contains(err.Error(), "WARD_KEY_A") {
		t.Errorf("expected the declared hint for vault a, got %v", err)
	}
	if !strings.Contains(err.Error(), "vault b") || !strings.Contains(err.Error(), "WARD_KEY_B") {
		t.Errorf("expected derived guidance for unhinted vault b, got %v", err)
	}
}

// SkippedVaults reports the vaults callers must not resolve paths against.
func TestSkippedVaults_reports_locked_vault(t *testing.T) {
	// arrange
	dir := t.TempDir()
	vA := filepath.Join(dir, "a")
	vB := filepath.Join(dir, "b")
	os.MkdirAll(vA, 0755)
	os.MkdirAll(vB, 0755)
	os.WriteFile(filepath.Join(vA, "main.ward"),
		[]byte("-----BEGIN AGE ENCRYPTED FILE-----\nxxx\n"), 0644)
	os.WriteFile(filepath.Join(vB, "conf.plain.ward"), []byte("b:\n  ok: \"1\"\n"), 0644)

	cfg := &config.Config{
		Encryption: config.Encryption{Engine: "age+armor"},
		Vaults: []config.Source{
			{Name: "a", Path: vA},
			{Name: "b", Path: vB},
		},
	}
	eng := NewEngine(cfg, sops.RequireKeyDecryptor{})

	// act
	if _, err := eng.MergeForView(); err != nil {
		t.Fatalf("expected b to load, got %v", err)
	}

	// assert
	got := eng.SkippedVaults()
	if len(got) != 1 || got[0] != "a" {
		t.Errorf("expected [a], got %v", got)
	}
}
