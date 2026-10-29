// Command dd-cli is the DiscoDrive terminal client.
package main

import (
	"os"

	"github.com/discodrive-cloud/discodrive-cli/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
