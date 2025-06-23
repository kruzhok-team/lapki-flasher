package main

import (
	"fmt"
	"time"
)

type FakeMS struct {
	portNames     [4]string
	fakeAddress   string // адрес, который генерируется при создании платы
	clientAddress string // адрес, указанный клиентом для прошивки, если не совпадает с fakeAddress, то выдаёт ошибку
}

func (board *FakeMS) GetSerialPort() string {
	return board.portNames[3]
}

func (board *FakeMS) IsConnected() bool {
	return board.portNames[0] != NOT_FOUND
}

func (board *FakeMS) Flash(filePath string, logger chan any) (string, error) {
	if board.fakeAddress != board.clientAddress {
		return "Address doesn't match", nil
	}
	time.Sleep(3 * time.Second)
	printLog(fmt.Sprintf("Fake uploading of file %s in board %v is completed", filePath, board))
	fakeMessage := "Fake flashing is completed"
	return fakeMessage, nil
}

func (board *FakeMS) GetWebMessageType() string {
	return MSDeviceMsg
}

func (board *FakeMS) GetWebMessage(name string, deviceID string) any {
	return MSDeviceMessage{
		ID:        deviceID,
		Name:      name,
		PortNames: board.portNames,
	}
}

func (board *FakeMS) Update() bool {
	return false
}

func (board *FakeMS) Ping() error {
	return nil
}

func (board *FakeMS) Reset() error {
	return nil
}

func (board *FakeMS) GetMetaData() (any, error) {
	return "fake metadata", nil
}
