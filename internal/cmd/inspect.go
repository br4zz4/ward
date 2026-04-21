package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/oporpino/ward/internal/secrets"
	"github.com/spf13/cobra"
)

func NewInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect [anchor.ward|dir]",
		Short: "Detect conflicts across all files (or within an anchor scope)",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			anchorPath := ""
			if len(args) == 1 {
				anchorPath = args[0]
			}

			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}

			if err := eng.Inspect(anchorPath); err == nil {
				fmt.Printf("%s✓%s no conflicts found\n", clrGreen, clrReset)
				return
			} else if ce, ok := err.(*secrets.ConflictError); ok {
				lines := strings.SplitN(ce.Error(), "\n", 2)
				fmt.Fprintf(os.Stderr, "%s%s%s\n", clrLightRed, lines[0], clrReset)
				if len(lines) > 1 {
					fmt.Fprintln(os.Stderr, lines[1])
				}
				os.Exit(1)
			} else {
				fatal(err)
			}
		},
	}
}
