package main

import (
	"os/exec"
	"time"
)

// прошивка, с автоматическим прописыванием необходимых параметров для avrdude
// ожидается, что плата заблокирована (board.IsFlashBlocked() == true)
func autoFlash(board *BoardToFlash, hexFilePath string) (avrdudeMessage string, err error) {
	portName := board.PortName
	if board.Type.hasBootloader() {
		if e := rebootPort(portName); e != nil {
			return "", e
		}
		board.refToBoot = refToBoot(board)
		portName = board.refToBoot.PortName
	}
	flashFile := "flash:w:" + getAbolutePath(hexFilePath) + ":a"
	// без опции "-D" не может прошить Arduino Mega
	args := []string{"-D", "-p", board.Type.Controller, "-c", board.Type.Programmer, "-P", portName, "-U", flashFile}
	if configPath != "" {
		args = append(args, "-C", configPath)
	}
	printLog(avrdudePath, args)
	flash(board, args)
	return
}

// прошивка через avrdude с аргументами, указанными в avrdudeArgs
func flash(board *BoardToFlash, avrdudeArgs []string) (avrdudeMessage string, err error) {
	cmd := exec.Command(avrdudePath, avrdudeArgs...)
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
