package main

import (
	"fmt"
	"os"
	"testing"
)

// loadSessions opens real local storage, so every package test gets disposable config.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "diffplay-test-config-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, key := range []string{"XDG_CONFIG_HOME", "HOME"} {
		if err = os.Setenv(key, dir); err != nil {
			os.RemoveAll(dir)
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
