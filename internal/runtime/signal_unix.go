//go:build unix

package runtime

import "syscall"

func syscallKill(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}
