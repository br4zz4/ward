package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func NewOverrideCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "override <file.ward>",
		Short: "Read YAML from stdin and encrypt it into the given .ward file",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			path := args[0]
			if err := requireWardFile(path); err != nil {
				fatal(err)
			}
			plain, err := io.ReadAll(os.Stdin)
			if err != nil {
				fatal(fmt.Errorf("reading stdin: %w", err))
			}
			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}
			if err := eng.Encrypt(path, plain); err != nil {
				fatal(fmt.Errorf("encrypting %s: %w", path, err))
			}
			fmt.Fprintf(os.Stderr, "ward: saved %s\n", path)
		},
	}
}
