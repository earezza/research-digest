//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

func hideWindow(cmd *exec.Cmd) {}

func showError(msg string) { fmt.Fprintln(os.Stderr, msg) }
