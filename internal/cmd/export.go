package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func NewExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export <file.ward> [output]",
		Short: "Decrypt a .ward file to stdout or to an output file",
		Args:  cobra.RangeArgs(1, 2),
		Run: func(_ *cobra.Command, args []string) {
			path := args[0]
			if err := requireWardFile(path); err != nil {
				fatal(err)
			}
			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}
			plain, err := eng.Decrypt(path)
			if err != nil {
				fatal(fmt.Errorf("decrypting %s: %w", path, err))
			}
			if len(args) == 2 {
				if err := os.WriteFile(args[1], plain, 0600); err != nil {
					fatal(fmt.Errorf("writing %s: %w", args[1], err))
				}
				fmt.Fprintf(os.Stderr, "ward: exported %s → %s\n", path, args[1])
			} else {
				os.Stdout.Write(plain)
			}
		},
	}
}
