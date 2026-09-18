package main

import (
	"os"

	"github.com/saleslumen/sl/internal/campaigns"
	"github.com/saleslumen/sl/internal/emails"
	"github.com/saleslumen/sl/internal/resources"
	"github.com/saleslumen/sl/internal/root"
	"github.com/saleslumen/sl/internal/script"
	"github.com/saleslumen/sl/internal/workflows"
)

func main() {
	os.Exit(root.Execute(campaigns.NewCommand, emails.NewCommand, workflows.NewCommand, script.NewCommand, resources.NewCommand))
}
