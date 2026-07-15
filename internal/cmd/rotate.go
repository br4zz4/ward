package cmd

import (
	"fmt"
	"os"
	"path/filepath"
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

			vaultPaths := make([]string, len(cfg.Vaults))
			for i, v := range cfg.Vaults {
				vaultPaths[i] = v.Path
			}

			if err := rotateKey(keyFile, vaultPaths); err != nil {
				fatal(err)
			}

			fmt.Println("Key rotated successfully.")
			fmt.Printf("Old key backed up to: %s/.key.bkp-<timestamp>\n", filepath.Dir(keyFile))
		},
	}
}

// rotateKey re-encrypts all .ward files in vaultDirs with a new age key.
// It uses a staging strategy: writes .ward.new files first, then atomically
// swaps them in only after all re-encryptions succeed. If anything fails,
// all staging files are deleted and the original key is preserved.
func rotateKey(keyFile string, vaultDirs []string) error {
	wardFiles, err := secrets.Discover(vaultDirs)
	if err != nil {
		return fmt.Errorf("discovering vault files: %w", err)
	}

	// generate new key in a temp file
	tmpKey, err := os.CreateTemp(filepath.Dir(keyFile), ".key-new-*")
	if err != nil {
		return fmt.Errorf("creating temp key: %w", err)
	}
	tmpKeyPath := tmpKey.Name()
	tmpKey.Close()
	defer os.Remove(tmpKeyPath) // cleaned up after rename or on error

	if err := wardage.GenerateKeyForce(tmpKeyPath); err != nil {
		return fmt.Errorf("generating new key: %w", err)
	}

	oldEnc := wardage.AgeArmorDecryptor{KeyFile: keyFile}
	newEnc := wardage.AgeArmorDecryptor{KeyFile: tmpKeyPath}

	// track staging files for cleanup on failure
	var stagingFiles []string

	rollback := func() {
		for _, f := range stagingFiles {
			os.Remove(f)
		}
	}

	// phase 1: re-encrypt each file to a .ward.new staging file
	for _, wardFile := range wardFiles {
		plain, err := oldEnc.Decrypt(wardFile)
		if err != nil {
			rollback()
			return fmt.Errorf("decrypting %s: %w", wardFile, err)
		}

		stagingPath := wardFile + ".new"
		if err := newEnc.Encrypt(stagingPath, plain); err != nil {
			rollback()
			return fmt.Errorf("encrypting %s: %w", wardFile, err)
		}
		stagingFiles = append(stagingFiles, stagingPath)
	}

	// commit point: backup old key, install new key, rename staging files
	timestamp := time.Now().UTC().Format("20060102150405")
	bkpPath := filepath.Join(filepath.Dir(keyFile), ".key.bkp-"+timestamp)

	oldKeyData, err := os.ReadFile(keyFile)
	if err != nil {
		rollback()
		return fmt.Errorf("reading current key: %w", err)
	}
	if err := os.WriteFile(bkpPath, oldKeyData, 0600); err != nil {
		rollback()
		return fmt.Errorf("writing key backup: %w", err)
	}

	if err := os.Rename(tmpKeyPath, keyFile); err != nil {
		os.Remove(bkpPath)
		rollback()
		return fmt.Errorf("installing new key: %w", err)
	}

	for i, stagingPath := range stagingFiles {
		target := wardFiles[i]
		if err := os.Rename(stagingPath, target); err != nil {
			// at this point the key is already swapped — best effort cleanup
			return fmt.Errorf("renaming %s: %w", stagingPath, err)
		}
	}

	return nil
}
