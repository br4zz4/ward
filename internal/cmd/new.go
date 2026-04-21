package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func NewNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <file.ward>",
		Short: "Create a new encrypted .ward file and open it in $EDITOR",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			path := args[0]

			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				fatal(fmt.Errorf("creating directory: %w", err))
			}

			if _, err := os.Stat(path); err == nil {
				fatal(fmt.Errorf("%s already exists — use `ward edit` to modify it", path))
			}

			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}

			stub := []byte("{}\n")
			if err := eng.Encrypt(path, stub); err != nil {
				fatal(fmt.Errorf("creating %s: %w", path, err))
			}

			plain, err := eng.Decrypt(path)
			if err != nil {
				fatal(fmt.Errorf("decrypting %s: %w", path, err))
			}

			tmp, err := writeTempFile(path, plain)
			if err != nil {
				fatal(err)
			}
			defer os.Remove(tmp)

			if err := openEditor(tmp); err != nil {
				fatal(err)
			}

			edited, err := os.ReadFile(tmp)
			if err != nil {
				fatal(fmt.Errorf("reading temp file: %w", err))
			}

			if err := eng.Encrypt(path, edited); err != nil {
				fatal(fmt.Errorf("re-encrypting %s: %w", path, err))
			}
		},
	}
}
