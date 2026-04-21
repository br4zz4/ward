package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func NewEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <file.ward>",
		Short: "Decrypt a .ward file, open in $EDITOR, re-encrypt on save",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			path := args[0]

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

			if err := cmd.Run(); err != nil {
				fatal(fmt.Errorf("editor exited with error: %w", err))
			}
		},
	}
}
