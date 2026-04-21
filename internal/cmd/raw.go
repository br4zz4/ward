package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func NewRawCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "raw <file.ward>",
		Short: "Print the decrypted YAML of a .ward file to the terminal",
		Args:  cobra.ExactArgs(1),
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
			os.Stdout.Write(plain)
		},
	}
}
