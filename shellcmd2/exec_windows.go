package shellcmd2

import (
	"os/exec"
)

func execCommand(c string) *exec.Cmd {
	return exec.Command("cmd.exe", "/C", c)
}