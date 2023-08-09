package main

import (
	"fmt"
	"os/exec"
)

const AVRDUDE = "avrdude"

// прошивка,
// ожидается, что плата заблокирована (board.IsFlashBlocked() == true)
func flash(board *BoardToFlash, filePath string) (avrdudeMessage string, err error) {
	flash := "flash:w:" + getAbolutePath(filePath) + ":a"
	fmt.Println(AVRDUDE, "-p", board.Type.Controller, "-c", board.Type.Programmer, "-P", board.PortName, "-U", flash)
	cmd := exec.Command(AVRDUDE, "-p", board.Type.Controller, "-c", board.Type.Programmer, "-P", board.PortName, "-U", flash)
	stdout, err := cmd.CombinedOutput()
	avrdudeMessage = string(stdout)
	return
}
