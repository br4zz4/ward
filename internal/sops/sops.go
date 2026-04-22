package sops

import (
	"fmt"
	"os"
	"time"

	sopslib "github.com/getsops/sops/v3"
	"github.com/getsops/sops/v3/aes"
	sopsage "github.com/getsops/sops/v3/age"
	"github.com/getsops/sops/v3/cmd/sops/common"
	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/getsops/sops/v3/config"
	"github.com/getsops/sops/v3/decrypt"
	"github.com/brazza-tech/ward/internal/age"
)

// SopsDecryptor decrypts and re-encrypts sops+age .ward files using the sops Go library.
type SopsDecryptor struct {
	KeyFile string
}

// Decrypt decrypts a sops+age YAML file and returns the plaintext YAML bytes.
func (d SopsDecryptor) Decrypt(path string) ([]byte, error) {
	if err := os.Setenv(sopsage.SopsAgeKeyFileEnv, d.KeyFile); err != nil {
		return nil, fmt.Errorf("setting %s: %w", sopsage.SopsAgeKeyFileEnv, err)
	}
	data, err := decrypt.File(path, "yaml")
	if err != nil {
		return nil, fmt.Errorf("sops decrypt %s: %w", path, err)
	}
	return data, nil
}

// Encrypt encrypts plaintext YAML and writes the sops+age file to path.
func (d SopsDecryptor) Encrypt(path string, plaintext []byte) error {
	pubKey, err := age.PublicKeyFrom(d.KeyFile)
	if err != nil {
		return err
	}

	masterKey, err := sopsage.MasterKeyFromRecipient(pubKey)
	if err != nil {
		return fmt.Errorf("building age master key: %w", err)
	}

	format := formats.FormatForPathOrString(path, "yaml")
	store := common.StoreForFormat(format, config.NewStoresConfig())

	branches, err := store.LoadPlainFile(plaintext)
	if err != nil {
		return fmt.Errorf("loading plaintext: %w", err)
	}

	tree := sopslib.Tree{
		Branches: branches,
		Metadata: sopslib.Metadata{
			KeyGroups: []sopslib.KeyGroup{
				{masterKey},
			},
			Version:            "3.12.2",
			LastModified:       time.Now().UTC(),
			UnencryptedSuffix:  "_unencrypted",
			EncryptedRegex:     "",
			UnencryptedRegex:   "",
			EncryptedSuffix:    "",
			ShamirThreshold:    0,
		},
	}

	dataKey, errs := tree.GenerateDataKey()
	if len(errs) > 0 {
		return fmt.Errorf("generating data key: %v", errs[0])
	}

	if err := common.EncryptTree(common.EncryptTreeOpts{
		DataKey: dataKey,
		Tree:    &tree,
		Cipher:  aes.NewCipher(),
	}); err != nil {
		return fmt.Errorf("encrypting tree: %w", err)
	}

	out, err := store.EmitEncryptedFile(tree)
	if err != nil {
		return fmt.Errorf("emitting encrypted file: %w", err)
	}

	return os.WriteFile(path, out, 0644)
}
