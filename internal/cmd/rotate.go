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
		Use:   "rotate-key",
		Short: "Rotate the encryption key and re-encrypt all vault files",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfgPath, err := resolvedConfigFile()
			if err != nil {
				fatal(fmt.Errorf("finding config: %w", err))
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				fatal(err)
			}
			keyFile, err := resolveKeyFile(cfg)
			if err != nil {
				fatal(err)
			}
			if keyFile == "" {
				fatal(fmt.Errorf("no encryption key configured"))
			}

			canonicalKeyPath := canonicalKeyFile(cfg)

			vaultPaths := make([]string, len(cfg.Vaults))
			for i, v := range cfg.Vaults {
				vaultPaths[i] = v.Path
			}

			bkpPath, err := rotateKey(keyFile, canonicalKeyPath, vaultPaths)
			if err != nil {
				fatal(err)
			}

			fmt.Println("Key rotated successfully.")
			fmt.Printf("Old key backed up to: %s\n", bkpPath)
		},
	}
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
// canonicalKeyPath is the on-disk file to back up and update (always .ward/.key or equivalent).
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
	bkpPath := filepath.Join(wardDir, ".key.bkp-"+timestamp)

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
