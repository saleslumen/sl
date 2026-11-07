package main

import (
	"embed"
	"os"

	"github.com/saleslumen/sl/internal/campaigns"
	"github.com/saleslumen/sl/internal/emails"
	"github.com/saleslumen/sl/internal/marketplace"
	"github.com/saleslumen/sl/internal/resources"
	"github.com/saleslumen/sl/internal/root"
	"github.com/saleslumen/sl/internal/script"
	"github.com/saleslumen/sl/internal/workflows"
)

//go:embed .agents/skills/*/SKILL.md
var bundledSkills embed.FS

func main() {
	os.Exit(root.Execute(bundledSkills, campaigns.NewCommand, emails.NewCommand, workflows.NewCommand, script.NewCommand, resources.NewCommand, marketplace.NewCommand))
}
