package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/algebananazzzzz/odyssey/internal/validate"
)

func main() {
	templates := flag.String("templates", ".", "path to a cicd-templates checkout")
	flag.Parse()

	switch flag.Arg(0) {
	case "validate":
		m, err := validate.Manifest(*templates + "/manifest.yml")
		if err != nil {
			fatal(err)
		}
		if err := validate.All(*templates+"/fragments", m); err != nil {
			fatal(err)
		}
		fmt.Println("manifest and fragments are valid")
	default:
		fmt.Fprintln(os.Stderr, "usage: odyssey-cli [--templates <dir>] validate")
		os.Exit(2)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "odyssey-cli:", err)
	os.Exit(1)
}
