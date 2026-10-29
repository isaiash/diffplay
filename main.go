package main

import (
	"fmt"
	"os"

	"github.com/isaiash/diffplay/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "diffplay:", err)
		os.Exit(1)
	}
}
