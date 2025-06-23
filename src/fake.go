package main

import (
	"fmt"
	"time"
)

// фальшивая плата, используется для тестирования, отправляется на клиент через тип сообщения "Device"
type FakeBoard struct {
	controller string
	programmer string
	serialID   string
	portName   string
}

// подключено ли устройство
func (board *FakeBoard) IsConnected() bool {
	return true
}

func (board *FakeBoard) GetSerialPort() string {
	return board.portName
}

func (board *FakeBoard) Flash(filePath string, logger chan any) (string, error) {
	time.Sleep(3 * time.Second)
	printLog(fmt.Sprintf("Fake uploading of file %s in board %v is completed", filePath, board))
	fakeMessage := "Fake flashing is completed"
	return fakeMessage, nil
}

func (board *FakeBoard) Update() bool {
	return false
}

func (board *FakeBoard) GetWebMessageType() string {
	return DeviceMsg
}

func (board *FakeBoard) GetWebMessage(name string, deviceID string) any {
	return DeviceMessage{
		ID:         deviceID,
		Name:       name,
		Controller: board.controller,
		Programmer: board.programmer,
		SerialID:   board.serialID,
		PortName:   board.portName,
	}
}

func (board *FakeBoard) Ping() error {
	return nil
}

func (board *FakeBoard) Reset() error {
	return nil
}

func (board *FakeBoard) GetMetaData() (any, error) {
	return "fake metadata", nil
}
