package main

import (
	"os/exec"
	"time"
)

func flashBootloader(board *BoardToFlash, hexFilePath string) (avrdudeMessage string, err error) {
	if e := rebootPort(board.PortName); e != nil {
		return "Не удалось перезагрузить порт", e
	}
	printLog("TEST1")
	detector.deviceWithBootloader <- board
	printLog("TEST2")
	go detector.Update()
	board = <-detector.bootloader
	printLog("TEST3")
	flashFile := "flash:w:" + getAbolutePath(hexFilePath) + ":a"
	// без опции "-D" не может прошить Arduino Mega
	args := []string{"-D", "-p", board.Type.Controller, "-c", board.Type.Programmer, "-P", board.PortName, "-U", flashFile}
	if configPath != "" {
		args = append(args, "-C", configPath)
	}
	printLog(avrdudePath, args)
	flash(board, args)
	return
}

// прошивка, с автоматическим прописыванием необходимых параметров для avrdude
// ожидается, что плата заблокирована (board.IsFlashBlocked() == true)
func autoFlash(board *BoardToFlash, hexFilePath string) (avrdudeMessage string, err error) {
	if board.Type.hasBootloader() {
		go flashBootloader(board, hexFilePath)
		return
	}
	printLog("TEST4")
	flashFile := "flash:w:" + getAbolutePath(hexFilePath) + ":a"
	// без опции "-D" не может прошить Arduino Mega
	args := []string{"-D", "-p", board.Type.Controller, "-c", board.Type.Programmer, "-P", board.PortName, "-U", flashFile}
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
