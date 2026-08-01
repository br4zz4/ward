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
