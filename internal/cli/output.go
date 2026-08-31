package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/algebananazzzzz/odyssey/internal/render"
)

func Checklist(p *render.Plan) string {
	if len(p.Github.Variables) == 0 && len(p.Github.Secrets) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("configure github before the first deploy (environment scope where prd differs):\n")
	for _, name := range p.Github.Variables {
		fmt.Fprintf(&b, "  gh variable set %s --body <value>\n", name)
	}
	for _, name := range p.Github.Secrets {
		fmt.Fprintf(&b, "  gh secret set %s\n", name)
	}
	return b.String()
}

func PrintBootstrap(p *render.Plan) string {
	if p.Bootstrap == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "bootstrap (%s):\n", p.Bootstrap.Intent)
	for _, cmd := range p.Bootstrap.Commands {
		fmt.Fprintf(&b, "  $ %s\n", cmd)
	}
	return b.String()
}

func RunBootstrap(p *render.Plan, dir string) error {
	if p.Bootstrap == nil {
		return nil
	}
	for _, line := range p.Bootstrap.Commands {
		cmd := exec.Command("sh", "-c", line)
		cmd.Dir = dir
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("bootstrap %q: %w", line, err)
		}
	}
	return nil
}
