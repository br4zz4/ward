package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/oporpino/ward/internal/secrets"
	"github.com/oporpino/ward/internal/sops"
	"github.com/spf13/cobra"
)

func NewEnvsCmd() *cobra.Command {
	var prefixed bool

	c := &cobra.Command{
		Use:   "envs [anchor.ward]",
		Short: "Show the env vars that would be injected by exec",
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

			tree, err := loadAndMerge(cfg, anchorPath)
			if err != nil {
				fatal(err)
			}

			var entries map[string]secrets.EnvEntry
			isDir := false
			if info, err := os.Stat(anchorPath); err == nil {
				isDir = info.IsDir()
			}

			if prefixed || anchorPath == "" {
				entries = secrets.ToEnvEntries(tree)
			} else if isDir {
				dec := sops.MockDecryptor{}
				dirFiles, err := secrets.Discover([]string{anchorPath})
				if err != nil || len(dirFiles) == 0 {
					entries = secrets.ToEnvEntries(tree)
				} else {
					ref, err := secrets.Load(dirFiles[0], dec)
					if err != nil {
						fatal(err)
					}
					entries = secrets.ToEnvEntriesFromAnchor(tree, ref.Data)
				}
			} else {
				dec := sops.MockDecryptor{}
				anchor, err := secrets.Load(anchorPath, dec)
				if err != nil {
					fatal(err)
				}
				entries = secrets.ToEnvEntriesFromAnchor(tree, anchor.Data)
			}

			keys := make([]string, 0, len(entries))
			for k := range entries {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool {
				oi := entries[keys[i]].Overrides
				oj := entries[keys[j]].Overrides
				if oi != oj {
					return oi
				}
				return keys[i] < keys[j]
			})

			maxLen := 0
			for _, k := range keys {
				if len(k) > maxLen {
					maxLen = len(k)
				}
			}

			ancestorLeafKeys := map[string]bool{}
			if anchorPath != "" {
				collectAncestorKeys(&secrets.Node{Children: tree}, anchorPath, ancestorLeafKeys)
			}

			for _, k := range keys {
				e := entries[k]
				padding := strings.Repeat(" ", maxLen-len(k))

				var keyColor string
				if e.Overrides || (anchorPath != "" && !isFromAnchorScope(e.Origin.File, anchorPath)) || ancestorLeafKeys[strings.ToLower(lastEnvSegment(k))] {
					keyColor = "\033[38;5;208m"
				} else {
					keyColor = "\033[32m"
				}

				fmt.Printf("%s%s%s%s  =  %s%v%s\n",
					keyColor, k, clrReset,
					padding,
					clrGray, e.Value, clrReset,
				)
			}

			fmt.Printf("\n%s%s●%s active  %s●%s overrides%s\n",
				clrGray, "\033[32m", clrGray,
				"\033[38;5;208m", clrGray,
				clrReset,
			)
		},
	}

	c.Flags().BoolVar(&prefixed, "prefixed", false, "show full path env var names including ancestor prefix")
	return c
}
