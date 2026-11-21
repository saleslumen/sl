package root

import (
	"fmt"
	"io/fs"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

var productSkills = []string{"campaigns", "emails", "workflows", "script", "resources", "marketplace"}

func newSkillCommand(f *cli.Factory, skills fs.FS) *cobra.Command {
	return &cobra.Command{
		Use:       "skill [PRODUCT]",
		Short:     "Print a public agent skill bundled with this sl version",
		Long:      "Print the general sl CLI skill, or a product skill for campaigns, emails, workflows, script, resources, or marketplace.",
		ValidArgs: productSkills,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("accepts at most one product")
			}
			if len(args) == 1 && !containsSkill(productSkills, args[0]) {
				return fmt.Errorf("unknown product %q; choose campaigns, emails, workflows, script, resources, or marketplace", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "sl"
			if len(args) == 1 {
				name = args[0]
			}
			markdown, err := fs.ReadFile(skills, ".agents/skills/"+name+"/SKILL.md")
			if err != nil {
				return fmt.Errorf("read bundled %s skill: %w", name, err)
			}
			if _, err := f.IO.Out.Write(markdown); err != nil {
				return fmt.Errorf("write bundled agent skill: %w", err)
			}
			return nil
		},
	}
}

func containsSkill(names []string, name string) bool {
	for _, candidate := range names {
		if name == candidate {
			return true
		}
	}
	return false
}
