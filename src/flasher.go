package main

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/polyus-nt/ms1-go/pkg/ms1"
)

var flasherSync sync.Mutex

// прошивка, с автоматическим прописыванием необходимых параметров для avrdude
// ожидается, что плата заблокирована (board.IsFlashBlocked() == true)
func autoFlash(board *BoardFlashAndSerial, filePath string) (flashMessage string, err error) {
	if board.isMSDevice() {
		return flashMS(board, filePath)
	}
	if board.Type.hasBootloader() {
		flasherSync.Lock()
		defer flasherSync.Unlock()
		if e := rebootPort(board.getPort()); e != nil {
			return "Не удалось перезагрузить порт", e
		}
		bootloaderType := board.Type.BootloaderTypeID
		detector.DontAddThisType(bootloaderType)
		defer detector.AddThisType(bootloaderType)
		defer time.Sleep(500 * time.Millisecond)
		var notAddedDevices map[string]*BoardFlashAndSerial
		found := false
		for i := 0; i < 25; i++ {
			// TODO: возможно стоит добавить количество необходимого времени в параметры сервера
			time.Sleep(500 * time.Millisecond)
			printLog("Попытка найти подходящее устройство", i+1)
			_, notAddedDevices, _ = detector.Update()
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
	flashFile := "flash:w:" + getAbolutePath(filePath) + ":a"
	// без опции "-D" не может прошить Arduino Mega
	args := []string{"-D", "-p", board.Type.Controller, "-c", board.Type.Programmer, "-P", board.getPort(), "-U", flashFile}
	if configPath != "" {
		args = append(args, "-C", configPath)
	}
	printLog(avrdudePath, args)
	return flash(args)
}

// прошивка через avrdude с аргументами, указанными в avrdudeArgs
func flash(avrdudeArgs []string) (avrdudeMessage string, err error) {
	cmd := exec.Command(avrdudePath, avrdudeArgs...)
	stdout, err := cmd.CombinedOutput()
	avrdudeMessage = handleFlashResult(string(stdout), err)
	return
}

// симуляция процесса прошивки, вместо неё, программа просто ждёт определённо время
func fakeFlash(board *BoardFlashAndSerial, filePath string) (fakeMessage string, err error) {
	time.Sleep(3 * time.Second)
	printLog(fmt.Sprintf("Fake uploading of file %s in board %v is completed", filePath, board))
	fakeMessage = "Fake flashing is completed"
	return
}

// прошивка МС-ТЮК
func flashMS(board *BoardFlashAndSerial, filePath string) (flashMessage string, err error) {
	port, err := ms1.MkSerial(board.getPortSync())
	if err != nil {
		return err.Error(), err
	}
	defer port.Close()

	device := ms1.NewDevice(port)
	_, err, b := device.GetId(true, true)
	if err != nil || b == false {
		return err.Error(), err
	}
	packs, err := device.WriteFirmware(filePath, true)
	if err != nil {
		return err.Error(), err
	}
	flashMessage = handleFlashResult(fmt.Sprint(packs), err)
	return
}

func handleFlashResult(flashOutput string, flashError error) (result string) {
	if flashError != nil {
		result = flashError.Error()
		if flashOutput != "" {
			result += "\n" + flashOutput
		}
	} else {
		result = flashOutput
	}
	return
}
