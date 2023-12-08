package main

import (
	"os/exec"
	"time"
)

// прошивка,
// ожидается, что плата заблокирована (board.IsFlashBlocked() == true)
func flash(board *BoardToFlash, hexFilePath string) (avrdudeMessage string, err error) {
	flash := "flash:w:" + getAbolutePath(hexFilePath) + ":a"
	// без опции "-D" не может прошить Arduino Mega
	args := []string{"-D", "-p", board.Type.Controller, "-c", board.Type.Programmer, "-P", board.PortName, "-U", flash}
	if configPath != "" {
		args = append(args, "-C", configPath)
	}
	printLog(avrdudePath, args)
	cmd := exec.Command(avrdudePath, args...)
	stdout, err := cmd.CombinedOutput()
	outputString := string(stdout)
	if err != nil {
		avrdudeMessage = err.Error()
		if outputString != "" {
			avrdudeMessage += "\n" + outputString
		}
	} else {
		avrdudeMessage = outputString
	}
	return
}

// симуляция процесса прошивки, вместо неё, программа просто ждёт определённо время
func fakeFlash(board *BoardToFlash, filePath string) (avrdudeMessage string, err error) {
	time.Sleep(3 * time.Second)
	avrdudeMessage = "Fake flashing is completed"
	return
}
