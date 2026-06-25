// Command omingest pushes database metadata to an OpenMetadata server, driven by a
// Python-compatible ingestion workflow YAML.
package main

import (
	"fmt"
	"os"

	"github.com/zerodha/openmetadata-ingestion-go/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
