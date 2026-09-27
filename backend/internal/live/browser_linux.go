//go:build linux

package live

import (
	"os/exec"
	"syscall"
)

func configureChromiumProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Its own process group, so stopping it takes every helper process along.
		Setpgid: true,
		// And if the server dies without stopping it, the kernel does.
		Pdeathsig: syscall.SIGKILL,
	}
}

func killChromium(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
