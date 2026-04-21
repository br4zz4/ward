package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const wardYAMLTemplate = `encryption:
  engine: sops+age
  key_env: WARD_AGE_KEY
  key_file: .ward.key

merge: merge

sources:
  - path: ./secrets
`

const wardFileTemplate = `# Add your secrets here.
# Structure is free — the YAML keys define the hierarchy.
#
# example:
# myapp:
#   database_url: "postgres://..."
#   redis_url: "redis://..."
`

func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate ward.yaml template and an initial secrets.ward file",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			if err := writeIfAbsent("ward.yaml", wardYAMLTemplate); err != nil {
				fatal(err)
			}
			if err := os.MkdirAll("secrets", 0755); err != nil {
				fatal(err)
			}
			if err := writeIfAbsent("secrets/secrets.ward", wardFileTemplate); err != nil {
				fatal(err)
			}
			fmt.Println("ward: initialized ward.yaml and secrets/secrets.ward")
		},
	}
}

func writeIfAbsent(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("ward: %s already exists, skipping\n", path)
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	fmt.Printf("ward: created %s\n", path)
	return nil
}
