//go:build !linux

package live

import "os/exec"

func configureChromiumProcess(*exec.Cmd) {}

func killChromium(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
