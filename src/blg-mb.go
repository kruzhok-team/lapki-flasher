package main

import (
	"bufio"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type BlgMb struct {
	serialID string
	version  string
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
	return BlgMbDeviceMessage{
		ID:       deviceID,
		Name:     name,
		SerialID: board.serialID,
		Version:  board.version,
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
	stdout, err := board.CyberBearLoader("load", "-f", filePath, "-b")
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

/*
Функция получения версии КиберМишки.

Автоматически обновляет поле version.
*/
func (board *BlgMb) GetVersion() (string, error) {
	if board.version != "" {
		return board.version, nil
	}
	value, err := board.GetMetaData()
	if err != nil {
		return "", err
	}
	meta, ok := value.(string)
	if !ok {
		return "", errors.New("ошибка преобразования данных при попытке получить версию КиберМишки")
	}

	scanner := bufio.NewScanner(strings.NewReader(meta))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "art:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				board.version = strings.TrimSpace(parts[1])
				return board.version, nil
			}
		}
	}

	return "", fmt.Errorf("art value not found")
}

// Извлечение прошивки
func (board *BlgMb) Extract() ([]byte, error) {
	_, err := board.CyberBearLoader("reboot", "-b")
	if err != nil {
		return []byte{}, err
	}
	_, err = board.CyberBearLoader("-m", "b1", "wait", "-t", "5")
	if err != nil {
		return []byte{}, err
	}
	bytes, err := board.CyberBearLoader("extract", "--pages", "44")
	if err != nil {
		return bytes, err
	}
	_, err = board.CyberBearLoader("reboot")
	if err != nil {
		// Предупреждаем, но не прекращаем работу
		printLog("Не удалось перезагрузить КиберМишку:", err.Error())
	}
	return bytes, nil
}

func (board *BlgMb) GetId() (string, error) {
	// TODO: унификация кода
	value, err := board.GetMetaData()
	if err != nil {
		return "", err
	}
	meta, ok := value.(string)
	if !ok {
		return "", errors.New("ошибка преобразования данных при попытке получить версию КиберМишки")
	}

	scanner := bufio.NewScanner(strings.NewReader(meta))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		printLog("scan", line)
		if strings.HasPrefix(line, "Serial:") {
			parts := strings.SplitN(line, ":", 2)
			printLog(parts, len(parts))
			if len(parts) == 2 {
				board.serialID = strings.TrimSpace(parts[1])
				printLog("serial id", board.serialID)
				return board.serialID, nil
			}
		}
	}

	return "", fmt.Errorf("serial value not found")
}
