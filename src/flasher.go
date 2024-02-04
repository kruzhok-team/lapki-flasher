package main

import (
	"errors"
	"os/exec"
	"sync"
	"time"
)

var flasherSync sync.Mutex

// прошивка, с автоматическим прописыванием необходимых параметров для avrdude
// ожидается, что плата заблокирована (board.IsFlashBlocked() == true)
func autoFlash(board *BoardToFlash, hexFilePath string) (avrdudeMessage string, err error) {
	if board.Type.hasBootloader() {
		flasherSync.Lock()
		defer flasherSync.Unlock()
		if e := rebootPort(board.PortName); e != nil {
			return "Не удалось перезагрузить порт", e
		}
		bootloaderType := board.Type.BootloaderTypeID
		detector.DontAddThisType(bootloaderType)
		defer detector.AddThisType(bootloaderType)
		defer time.Sleep(500 * time.Millisecond)
		var notAddedDevices map[string]*BoardToFlash
		found := false
		for i := 0; i < 25; i++ {
			// TODO: возможно стоит добавить количество необходимого времени в параметры сервера
			time.Sleep(500 * time.Millisecond)
			printLog("Попытка найти подходящее устройство", i+1)
			_, _, _, _, notAddedDevices = detector.Update()
			sameTypeCnt := 0
			for _, dev := range notAddedDevices {
				if dev.Type.typeID == bootloaderType {
					board = dev
					sameTypeCnt++
					if sameTypeCnt > 1 {
						return "Не удалось опознать Bootloader. Ошибка могла быть вызвана перезагрузкой одного из устройств, либо из-за подключения нового.", errors.New("bootloader: too many")
					}
					found = true
				}
			}
			if found {
				break
			}
		}
		if !found {
			return "Не удалось найти Bootloader.", errors.New("bootloader: not found")
		}
	}
	flashFile := "flash:w:" + getAbolutePath(hexFilePath) + ":a"
	// без опции "-D" не может прошить Arduino Mega
	args := []string{"-D", "-p", board.Type.Controller, "-c", board.Type.Programmer, "-P", board.PortName, "-U", flashFile}
	if configPath != "" {
		args = append(args, "-C", configPath)
	}
	printLog(avrdudePath, args)
	return flash(board, args)
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
