package main

import (
	"fmt"
	"os"

	"joblet/internal/rnx"
)

func main() {
	if err := rnx.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
