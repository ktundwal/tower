package main

import (
	"context"
	"fmt"
	"os"

	"tower/internal/app"
)

func main() {
	if err := app.RunDemo(context.Background(), os.Stdout, os.Stderr, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tower-demo:", err)
		os.Exit(1)
	}
}
