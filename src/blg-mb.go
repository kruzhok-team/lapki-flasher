package main

import (
	"errors"
	"os/exec"
)

type BlgMb struct {
}

func (board *BlgMb) IsConnected() bool {
	return board.Ping() == nil
}

func (board *BlgMb) GetSerialPort() string {
	return ""
}

func (board *BlgMb) GetWebMessageType() string {
	return BlgMbDeviceMsg
}

func (board *BlgMb) GetWebMessage(name string, deviceID string) any {
	return SimpleDeviceMessage{
		ID:   deviceID,
		Name: name,
	}
}

func (board *BlgMb) Flash(filePath string, logger chan any) (string, error) {
	cmd := exec.Command(blgMbUploaderPath, "-w", "-f", filePath)
	stdout, err := cmd.CombinedOutput()
	msg := handleFlashResult(string(stdout), err)
	return msg, err
}

func (board *BlgMb) Ping() error {
	cmd := exec.Command(blgMbUploaderPath, "-p")
	return cmd.Run()
}

func (board *BlgMb) Reset() error {
	return errors.New("операция перезагрузки не поддерживается для этого устройства")
}

func (board *BlgMb) Update() bool {
	return false
}
