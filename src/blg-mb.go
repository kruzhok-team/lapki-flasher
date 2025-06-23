package main

import (
	"os/exec"
)

type BlgMb struct {
	serialID string
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

func (board *BlgMb) CyberBearLoader(args ...string) ([]byte, error) {
	if board.serialID != "" {
		targetArgs := []string{"-t", board.serialID}
		args = append(targetArgs, args...)
	}
	cmd := exec.Command(blgMbUploaderPath, args...)
	return cmd.CombinedOutput()
}

func (board *BlgMb) Flash(filePath string, logger chan any) (string, error) {
	stdout, err := board.CyberBearLoader("-m", "b1", "load", "-f", filePath)
	msg := handleFlashResult(string(stdout), err)
	return msg, err
}

func (board *BlgMb) Ping() error {
	_, err := board.CyberBearLoader("identify")
	return err
}

func (board *BlgMb) Reset() error {
	_, err := board.CyberBearLoader("reboot")
	return err
}

func (board *BlgMb) Update() bool {
	return false
}

func (board *BlgMb) GetMetaData() (any, error) {
	stdout, stderr := board.CyberBearLoader("identify")
	return string(stdout), stderr
}
