package main

import (
	"fmt"
	"os"

	"joblet/internal/rnx/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
