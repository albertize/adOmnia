//go:build !windows

package extensions

import "os/exec"

func configureExtensionHostCommand(_ *exec.Cmd) {}
