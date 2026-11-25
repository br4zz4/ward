package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	wardage "github.com/br4zz4/ward/internal/age"
	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/secrets"
)

func NewRotateKeyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rotate-key [vault]",
		Short: "Rotate the encryption key and re-encrypt vault files",
		Long: `Rotate the encryption key and re-encrypt vault files.

With a vault, only that vault's key is rotated and only its files are
re-encrypted. Without one, the project's single key is rotated; a project with
several distinct keys must name the vault.`,
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(c *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return completeVaultNames(c, args, toComplete)
		},
		Run: func(_ *cobra.Command, args []string) {
			cfgPath, err := resolvedConfigFile()
			if err != nil {
				fatal(fmt.Errorf("finding config: %w", err))
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				fatal(err)
			}
			if len(args) == 1 {
				rotateOneVault(cfg, args[0])
				return
			}
			rotateProjectKey(cfg)
		},
	}
}

// rotateProjectKey keeps the single-key behaviour: every vault shares one key.
func rotateProjectKey(cfg *config.Config) {
	if names := distinctKeyVaults(cfg); len(names) > 1 {
		fatal(fmt.Errorf("this project has more than one key (vaults: %s) — rotate one with `ward rotate-key <vault>`",
			strings.Join(names, ", ")))
	}
	keyFile, err := resolveKeyFile(cfg)
	if err != nil {
		fatal(err)
	}
	if keyFile == "" {
		fatal(fmt.Errorf("no encryption key configured"))
	}

	vaultPaths := make([]string, len(cfg.Vaults))
	for i, v := range cfg.Vaults {
		vaultPaths[i] = v.Path
	}

	canonical := canonicalKeyFile(cfg)
	bkpPath, err := rotateKey(keyFile, canonical, vaultPaths)
	if err != nil {
		fatal(err)
	}
	if err := mirrorKeyFiles(canonical, vaultNames(cfg)...); err != nil {
		fatal(err)
	}
	fmt.Println("Key rotated successfully.")
	fmt.Printf("Old key backed up to: %s\n", bkpPath)
	protectBackups(canonical)
}

// distinctKeyVaults returns the vault names when their effective key files
// differ (a vault's own, else the global one); nil when they all share one.
func distinctKeyVaults(cfg *config.Config) []string {
	seen := map[string]bool{}
	var names []string
	for _, v := range cfg.Vaults {
		eff := v.Encryption.KeyFile
		if eff == "" {
			eff = cfg.Encryption.KeyFile
		}
		seen[eff] = true
		names = append(names, v.CanonicalName())
	}
	if len(seen) <= 1 {
		return nil
	}
	return names
}

// rotateOneVault rotates a single vault's key file and re-encrypts only its files.
func rotateOneVault(cfg *config.Config, name string) {
	v := findVault(cfg, name)
	if v == nil {
		fatalVaultNotFound(name)
	}
	keyPath := v.Encryption.KeyFile
	if keyPath == "" {
		if _, envToken, _ := vaultEnvToken(v.Name); envToken == "" && cfg.Encryption.KeyFile != "" {
			fatal(fmt.Errorf("vault %s uses the project key %s — run `ward rotate-key` without a vault", v.CanonicalName(), cfg.Encryption.KeyFile))
		}
		fatal(fmt.Errorf("rotate-key requires a key file for vault %s — the key is only available via env, so the new key would have nowhere to be stored", v.CanonicalName()))
	}
	for i := range cfg.Vaults {
		other := &cfg.Vaults[i]
		if other.CanonicalName() != v.CanonicalName() && other.Encryption.KeyFile == keyPath {
			fatal(fmt.Errorf("key file %s is shared by vaults %s and %s — rotating it would lock %s out", keyPath, v.CanonicalName(), other.CanonicalName(), other.CanonicalName()))
		}
	}
	if _, err := os.Stat(keyPath); err != nil {
		fatal(fmt.Errorf("rotate-key requires a key file for vault %s — %s not found", v.CanonicalName(), keyPath))
	}
	current, err := resolveKeyFileForVault(*v)
	if err != nil {
		fatal(err)
	}
	if current == "" {
		fatal(fmt.Errorf("no encryption key configured for vault %s", v.CanonicalName()))
	}

	bkpPath, err := rotateKey(current, keyPath, []string{v.Path})
	if err != nil {
		fatal(err)
	}

	if err := mirrorKeyFiles(keyPath, v.Name); err != nil {
		fatal(err)
	}
	newKey, err := os.ReadFile(keyPath)
	if err != nil {
		fatal(fmt.Errorf("reading new key: %w", err))
	}

	fmt.Printf("Key for vault %s rotated successfully.\n", v.CanonicalName())
	fmt.Printf("Old key backed up to: %s\n", bkpPath)
	if IsAIMode() {
		printKeyHiddenNotice(keyPath)
		protectBackups(keyPath)
		return
	}
	token := strings.TrimSpace(string(newKey))
	if !strings.HasPrefix(token, "ward-") {
		token = "ward-" + base64.URLEncoding.EncodeToString(newKey)
	}
	envs := config.KeyEnvNames(v.Name)
	fmt.Printf("Update your CI secret:\n  %s=%s\n", envs[0], token)
	if len(envs) > 1 {
		fmt.Printf("  (%s is accepted as well)\n", envs[1])
	}
	protectBackups(keyPath)
}

func protectBackups(keyPath string) {
	pattern := filepath.ToSlash(filepath.Join(filepath.Dir(keyPath), "*.bkp-*"))
	if err := ensureGitignore(pattern); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not add %s to .gitignore — do not commit key backups: %v\n", pattern, err)
	}
}

func vaultNames(cfg *config.Config) []string {
	names := make([]string, len(cfg.Vaults))
	for i, v := range cfg.Vaults {
		names[i] = v.Name
	}
	return names
}

func mirrorKeyFiles(canonical string, names ...string) error {
	newKey, err := os.ReadFile(canonical)
	if err != nil {
		return fmt.Errorf("reading new key: %w", err)
	}
	for _, name := range names {
		for _, cand := range config.KeyFileCandidates(name) {
			if cand == canonical {
				continue
			}
			if _, err := os.Stat(cand); err != nil {
				continue
			}
			if err := os.WriteFile(cand, newKey, 0600); err != nil {
				return fmt.Errorf("updating %s: %w", cand, err)
			}
		}
	}
	return nil
}

// canonicalKeyFile returns the path where the key is stored on disk (from config or default).
// This is the file that gets backed up and updated — as opposed to the resolved temp file
// that resolveKeyFile may return when the key is stored as a ward- token.
func canonicalKeyFile(cfg *config.Config) string {
	if cfg.Encryption.KeyFile != "" {
		return cfg.Encryption.KeyFile
	}
	return config.DefaultKeyFile
}

// rotateKey re-encrypts all .ward files in vaultDirs with a new age key.
//
// keyFile is the resolved key (may be a temp file when the canonical stores a ward- token).
// canonicalKeyPath is the on-disk file to back up and update (e.g. .ward/<vault>.key).
//
// Strategy: write .ward.new staging files first, then at the commit point backup the canonical
// key, install the new raw key there, and rename all staging files. Rolls back on any failure
// before the commit point.
//
// Returns the path of the old key backup file on success.
func rotateKey(keyFile, canonicalKeyPath string, vaultDirs []string) (string, error) {
	wardFiles, err := secrets.Discover(vaultDirs)
	if err != nil {
		return "", fmt.Errorf("discovering vault files: %w", err)
	}

	// generate new key in a temp file inside the .ward dir (same filesystem as canonical)
	wardDir := filepath.Dir(canonicalKeyPath)
	tmpKey, err := os.CreateTemp(wardDir, ".key-new-*")
	if err != nil {
		return "", fmt.Errorf("creating temp key: %w", err)
	}
	tmpKeyPath := tmpKey.Name()
	tmpKey.Close()
	defer os.Remove(tmpKeyPath) // no-op after rename succeeds

	if err := wardage.GenerateKeyForce(tmpKeyPath); err != nil {
		return "", fmt.Errorf("generating new key: %w", err)
	}

	oldEnc := wardage.AgeArmorDecryptor{KeyFile: keyFile}
	newEnc := wardage.AgeArmorDecryptor{KeyFile: tmpKeyPath}

	var stagingFiles []string
	rollback := func() {
		for _, f := range stagingFiles {
			os.Remove(f)
		}
	}

	// re-encrypt each file to a .ward.new staging file
	for _, wardFile := range wardFiles {
		plain, err := oldEnc.Decrypt(wardFile)
		if err != nil {
			rollback()
			return "", fmt.Errorf("decrypting %s: %w", wardFile, err)
		}

		stagingPath := wardFile + ".new"
		if err := newEnc.Encrypt(stagingPath, plain); err != nil {
			rollback()
			return "", fmt.Errorf("encrypting %s: %w", wardFile, err)
		}
		stagingFiles = append(stagingFiles, stagingPath)
	}

	// commit point: backup canonical key, install new raw key, rename staging files
	timestamp := time.Now().UTC().Format("20060102150405")
	bkpPath := canonicalKeyPath + ".bkp-" + timestamp

	canonicalData, err := os.ReadFile(canonicalKeyPath)
	if err != nil {
		rollback()
		return "", fmt.Errorf("reading key file: %w", err)
	}
	if err := os.WriteFile(bkpPath, canonicalData, 0600); err != nil {
		rollback()
		return "", fmt.Errorf("writing key backup: %w", err)
	}

	// install new key as raw age key (not token) — consistent regardless of original format
	newKeyData, err := os.ReadFile(tmpKeyPath)
	if err != nil {
		os.Remove(bkpPath)
		rollback()
		return "", fmt.Errorf("reading new key: %w", err)
	}

	// if canonical was a ward- token, write the new key as a token too
	if strings.HasPrefix(strings.TrimSpace(string(canonicalData)), "ward-") {
		token := "ward-" + base64.URLEncoding.EncodeToString(newKeyData)
		newKeyData = []byte(token + "\n")
	}

	if err := os.WriteFile(canonicalKeyPath, newKeyData, 0600); err != nil {
		os.Remove(bkpPath)
		rollback()
		return "", fmt.Errorf("installing new key: %w", err)
	}

	for i, stagingPath := range stagingFiles {
		target := wardFiles[i]
		if err := os.Rename(stagingPath, target); err != nil {
			// key is already swapped — best effort cleanup only
			return "", fmt.Errorf("renaming %s: %w", stagingPath, err)
		}
	}

	return bkpPath, nil
}
