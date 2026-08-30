package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/algebananazzzzz/odyssey/internal/engine"
)

func main() {
	templates := flag.String("templates", ".", "path to a cicd-templates checkout")
	flag.Parse()

	switch flag.Arg(0) {
	case "validate":
		m, err := engine.LoadManifest(*templates + "/manifest.yml")
		if err != nil {
			fatal(err)
		}
		frag := os.DirFS(*templates + "/fragments")
		if err := engine.ValidateAll(frag, m); err != nil {
			fatal(err)
		}
		fmt.Println("all combinations render and validate")
	default:
		fmt.Fprintln(os.Stderr, "usage: odyssey-cli [--templates <dir>] validate")
		os.Exit(2)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "odyssey-cli:", err)
	os.Exit(1)
}
