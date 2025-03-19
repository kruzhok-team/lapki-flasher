package main

import "errors"

type BlgMb struct {
}

func (board *BlgMb) IsConnected() bool {
	return board.Ping() == nil
}

func (board *BlgMb) GetSerialPort() string {
	return ""
}

func (board *BlgMb) GetWebMessageType() string {
	// TODO
	return ""
}

func (board *BlgMb) GetWebMessage(name string, deviceID string) any {
	// TODO
	return DeviceMessage{
		ID:   deviceID,
		Name: name,
	}
}

func (board *BlgMb) Flash(filePath string, logger chan any) (string, error) {
	// TODO
	return "", nil
}

func (board *BlgMb) Ping() error {
	return errors.New("операция пинг не поддерживается для этого устройства")
}

func (board *BlgMb) Reset() error {
	return errors.New("операция перезагрузки не поддерживается для этого устройства")
}

func (board *BlgMb) Update() bool {
	return false
}
