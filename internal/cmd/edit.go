package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func NewEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit [file.ward]",
		Short: "Decrypt a .ward file, open in $EDITOR, re-encrypt on save",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			var path string
			if len(args) == 1 {
				path = args[0]
			} else {
				cfg, err := loadConfig()
				if err != nil || len(cfg.Sources) == 0 {
					fatal(fmt.Errorf("no file specified and no sources configured"))
				}
				path = cfg.Sources[0].Path
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}

			// TODO: decrypt to temp file, open editor, re-encrypt
			// For now open the file directly (plain text, pre-SOPS integration)
			cmd := exec.Command(editor, path)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			err := cmd.Run()
			if err != nil {
				info, statErr := os.Stat(path)
				if statErr == nil && info.IsDir() {
					// vim exits 1 when opening a directory via netrw — not a real error
					return
				}
				fatal(fmt.Errorf("editor exited with error: %w", err))
			}
		},
	}
}
