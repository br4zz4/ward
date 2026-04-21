package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/oporpino/ward/internal/config"
	"github.com/oporpino/ward/internal/secrets"
	"github.com/oporpino/ward/internal/sops"
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

			cfg, err := loadConfig()
			if err != nil {
				fatal(err)
			}

			dec := sops.MockDecryptor{}
			paths, err := secrets.Discover(sourcePaths(cfg))
			if err != nil {
				fatal(err)
			}
			files, err := secrets.LoadAll(paths, dec)
			if err != nil {
				fatal(err)
			}

			ordered := buildOrderedFiles(cfg, anchorPath, files)
			if ordered == nil {
				fatal(fmt.Errorf("anchor not found: %s", anchorPath))
			}

			_, mergeErr := secrets.Merge(ordered, config.MergeModeError)
			if mergeErr == nil {
				fmt.Printf("%s✓%s no conflicts found\n", clrGreen, clrReset)
				return
			}

			ce, ok := mergeErr.(*secrets.ConflictError)
			if !ok {
				fatal(mergeErr)
			}

			lines := strings.SplitN(ce.Error(), "\n", 2)
			fmt.Fprintf(os.Stderr, "%s%s%s\n", clrLightRed, lines[0], clrReset)
			if len(lines) > 1 {
				fmt.Fprintln(os.Stderr, lines[1])
			}
			os.Exit(1)
		},
	}
}
