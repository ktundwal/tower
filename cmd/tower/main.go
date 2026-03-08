package main

import (
	"context"
	"fmt"
	"os"

	"tower/internal/app"
)

func main() {
	if err := app.RunCLI(context.Background(), os.Stdout, os.Stderr, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tower:", err)
		os.Exit(1)
	}
}
