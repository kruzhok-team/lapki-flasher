package main

import (
	"fmt"

	"github.com/polyus-nt/ms1-go/pkg/ms1"
)

type MS1 struct {
	portNames []string
	address   string
	verify    bool // если true, то будет проверка после прошивки
	ms1OS     MS1OS
}

func (board *MS1) GetSerialPort() string {
	return board.portNames[3]
}

func (board *MS1) IsConnected() bool {
	return board.portNames[0] != NOT_FOUND
}

func (board *MS1) Flash(filePath string) (string, error) {
	port, err := ms1.MkSerial(board.getFlashPort())
	if err != nil {
		return err.Error(), err
	}
	defer port.Close()

	device := ms1.NewDevice(port)
	if board.address != "" {
		err := device.SetAddress(board.address)
		if err != nil {
			return "Не удалось использовать адрес устройства. " + err.Error(), err
		}
	}
	packs, err := device.WriteFirmware(filePath, board.verify)
	if err != nil {
		return err.Error(), err
	}
	flashMessage := handleFlashResult(fmt.Sprint(packs), err)
	return flashMessage, err
}

func (board *MS1) getFlashPort() string {
	return board.portNames[0]
}
