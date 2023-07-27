package main

import (
	"os/exec"
)

const AVRDUDE = "avrdude"

// прошивка
func flash(board *BoardToFlash, filePath string) (avrdudeMessage string, err error) {
	flash := "flash:w:" + getAbolutePath(filePath) + ":a"
	cmd := exec.Command(AVRDUDE, "-p", board.Type.Controller, "-c", board.Type.Programmer, "-P", board.PortName, "-U", flash)
	stdout, err := cmd.CombinedOutput()
	avrdudeMessage = string(stdout)
	return
}
