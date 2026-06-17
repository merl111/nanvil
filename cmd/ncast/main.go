package main

import (
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/ncast/cli"
)

func main() {
	app := cli.NewApp()
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
