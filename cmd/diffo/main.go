package main

import (
	"fmt"
	"os"

	"diffo/internal/app"
)

func main() {
	if err := app.Run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
