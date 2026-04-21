package cmd

import (
	"github.com/spf13/cobra"
)

func NewConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Open the ward config in $EDITOR",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			path, err := resolvedConfigFile()
			if err != nil {
				fatal(err)
			}
			if err := openEditor(path); err != nil {
				fatal(err)
			}
		},
	}
}
