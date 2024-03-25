//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type USBdevice struct {
	Name  string      `json:"_name"`
	PID   string      `json:"product_id"`
	VID   string      `json:"vendor_id"`
	LID   string      `json:"location_id"`
	Items []USBdevice `json:"_items"`
}
type USBJSONARRAY struct {
	SPUSBDataType []USBdevice `json:"SPUSBDataType"`
}

func extractID(input string) (string, error) {
	id := strings.Fields(input)[0]
	if !strings.Contains(id, "0x") {
		return "", fmt.Errorf("Failed to extract ID")
	}
	return id[2:], nil
}

// настройка ОС (для Darwin она не требуется, но она здесь присутствует, чтобы обеспечить совместимость с другими платформами, которые использует свои реализации этой функции)
func setupOS() {}

// находит все подключённые платы
// TODO: добавить поиск сериного номера
func detectBoards(boardTemplates []BoardTemplate) map[string]*BoardToFlash {
	boards := make(map[string]*BoardToFlash)
	cmd := exec.Command("system_profiler", "SPUSBDataType", "-JSON")
	jsonData, err := cmd.CombinedOutput()
	if err != nil {
		printLog("system_profiler error", string(jsonData), err.Error())
		return nil
	}
	jsonArr := USBJSONARRAY{}
	err = json.Unmarshal([]byte(jsonData), &jsonArr)
	if err != nil {
		printLog("JSON unmarshal error:", err.Error(), cmd.String())
		printLog(string(jsonData))
		return nil
	}
	for _, boardTemplate := range boardTemplates {
		for _, vid := range boardTemplate.VendorIDs {
			for _, pid := range boardTemplate.ProductIDs {
				deviceID, err := searchNameLocationID(jsonArr.SPUSBDataType, pid, vid)
				if err != nil || deviceID == "" {
					continue
				}
				boardType := BoardType{
					typeID:           boardTemplate.ID,
					ProductID:        pid,
					VendorID:         vid,
					Name:             boardTemplate.Name,
					Controller:       boardTemplate.Controller,
					Programmer:       boardTemplate.Programmer,
					BootloaderTypeID: boardTemplate.BootloaderID,
				}
				detectedBoard := NewBoardToFlash(boardType, NOT_FOUND)
				detectedBoard.updatePortName(deviceID)
				if detectedBoard.getPortSync() == NOT_FOUND {
					printLog("Failed to find port")
					continue
				}
				boards[deviceID] = detectedBoard
				printLog("Device was found:", detectedBoard, deviceID)
			}
		}
	}
	return boards
}

// true - если порт изменился или не найден, иначе false
// назначает порту значение NOT_FOUND, если не удалось найти порт
// TODO: переделать интерфейс функции для все платформ, сделать, чтобы функция возвращала error
func (board *BoardToFlash) updatePortName(ID string) bool {
	// ioreg -r -c IOUSBHostDevice -l -n 'QT2040 Trinkey' | grep -Ei 'class.IO|ttydevice|tty.usbmodem|@'
	ultimate := "ioreg -r -c IOUSBHostDevice -l -n " + "\"" + ID + "\"" + " | grep IODialinDevice"
	cmd := exec.Command("/bin/sh", "-c", ultimate)
	//cmd := exec.Command("ioreg", "-r", "-c", "IOUSBHostDevice", "-l", "-n", ID, "grep IODialinDevice")
	var output bytes.Buffer
	cmd.Stdout = &output
	err := cmd.Run()
	if err != nil {
		printLog("Failed to run cmd", cmd.String())
		printLog("Error:", err.Error())
		board.setPortSync(NOT_FOUND)
		return true
	}
	strRes := output.String()
	fields := strings.Fields(strRes)
	if len(fields) != 3 {
		printLog("CMD:", cmd.String())
		printLog("Unable to extract port name from:", strRes)
		board.setPortSync(NOT_FOUND)
		return true
	}
	oldPortName := board.getPortSync()
	newPortName := strings.Trim(fields[2], "\"")
	if oldPortName != newPortName {
		board.setPortSync(newPortName)
		return true
	}
	return false
}

// возращает имя устройства и location_id: name@location_id. Является ключом устройства для загрузчика
func searchNameLocationID(devices []USBdevice, PID string, VID string) (string, error) {
	for _, dev := range devices {
		//println(dev.Name, len(dev.Items), dev.PID, dev.VID)
		if dev.Items != nil {
			rec, err := searchNameLocationID(dev.Items, PID, VID)
			if err == nil {
				return rec, nil
			}
		}
		if dev.PID == "" || dev.VID == "" {
			continue
		}
		pid, err := extractID(dev.PID)
		if err != nil {
			continue
		}
		vid, err := extractID(dev.VID)
		if err != nil {
			continue
		}
		if PID == pid && VID == vid {
			lid, err := extractID(dev.LID)
			if err != nil {
				continue
			}
			return dev.Name + "@" + lid, nil
		}
	}
	return "", fmt.Errorf("location_id is not found")
}
