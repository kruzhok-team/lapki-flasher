package main

import (
	"fmt"

	"github.com/polyus-nt/ms1-go/pkg/ms1"
)

type MS1 struct {
	portNames [4]string // должно быть ровно 4 порта
	address   string
	verify    bool  // если true, то будет проверка после прошивки
	ms1OS     MS1OS // структура с данными для поиска устройства на определённой ОС
}

func NewMS1(portNames [4]string, ms1OS MS1OS) *MS1 {
	ms1 := MS1{
		portNames: portNames,
		address:   "",
		verify:    false,
		ms1OS:     ms1OS,
	}
	return &ms1
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

func (board *MS1) GetWebMessageType() string {
	return MSDeviceMsg
}

func (board *MS1) GetWebMessage(name string, deviceID string) any {
	return MSDeviceMessage{
		ID:        deviceID,
		Name:      name,
		PortNames: board.portNames,
	}
}

func (board *MS1) reset() error {
	portMS, err := ms1.MkSerial(board.getFlashPort())
	if err != nil {
		return err
	}
	defer portMS.Close()
	deviceMS := ms1.NewDevice(portMS)
	err = deviceMS.SetAddress(board.address)
	if err != nil {
		return err
	}
	deviceMS.Reset(true)
	return nil
}

func (board *MS1) ping() error {
	portMS, err := ms1.MkSerial(board.getFlashPort())
	if err != nil {
		return err
	}
	defer portMS.Close()
	deviceMS := ms1.NewDevice(portMS)
	err = deviceMS.SetAddress(board.address)
	_, err = deviceMS.Ping()
	if err != nil {
		return err
	}
	return nil
}
