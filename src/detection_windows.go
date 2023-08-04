//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

// возвращает пути к устройствам, согласно заданному шаблону, если они есть, иначе возвращает пустой массив (nil)
func getInstanceId(path string) []string {
	start := time.Now()
	cmd := fmt.Sprintf("Get-PnpDevice -PresentOnly -InstanceId '%s' | Select-Object -Property InstanceId", path)
	cmdExec := exec.Command("powershell", cmd)
	cmdResult, err := cmdExec.CombinedOutput()
	var possiblePathes []string
	// значит такое устройство не подключено
	if err != nil {
		end := time.Now()
		fmt.Println("get instance", end.Sub(start))
		return possiblePathes
	}

	// получение возможных путей к устройствам

	curStr := ""
	for _, v := range cmdResult {
		if v == '\n' {
			if len(curStr) > 0 && curStr[0] == 'U' {
				possiblePathes = append(possiblePathes, curStr)
			}
			curStr = ""
			continue
		}
		// Carriage Return code = 13
		if v == 13 {
			continue
		}
		curStr += string(v)
	}
	end := time.Now()
	fmt.Println("get instance", end.Sub(start))
	return possiblePathes
}

// находит все подключённые платы, считает все найденные устройства за новые
func detectBoards() map[string]*BoardToFlash {
	startTime := time.Now()
	vendors := vendorList()
	boardTypes := boardList()
	boards := make(map[string]*BoardToFlash)
	presentUSBDevices := getInstanceId("USB*")
	// нет usb-устройств
	if presentUSBDevices == nil {
		return nil
	}
	for _, line := range presentUSBDevices {
		device := strings.TrimSpace(line)
		deviceLen := len(device)
		for _, vendor := range vendors {
			for _, boardType := range boardTypes[vendor] {
				pathPattern := fmt.Sprintf("USB\\VID_%s&PID_%s", vendor, boardType.ProductID)
				pathLen := len(pathPattern)
				// нашли подходящее устройство
				if pathLen <= deviceLen && strings.ToLower(device[:pathLen]) == strings.ToLower(pathPattern) {
					portName := findPortName(&device)
					if portName == NOT_FOUND {
						fmt.Println(device)
						continue
					}
					boardType.VendorID = vendor
					detectedBoard := NewBoardToFlash(boardType, portName)
					serialIndex := strings.LastIndex(device, "\\")
					possibleSerialID := device[serialIndex+1:]
					if !strings.Contains(possibleSerialID, "&") {
						detectedBoard.SerialID = device[serialIndex+1:]
					}
					boards[device] = detectedBoard
				}
			}
		}
	}
	endTime := time.Now()
	fmt.Println("Detection time: ", endTime.Sub(startTime))
	return boards
}

// возвращает имя порта, либо константу NOT_FOUND, если не удалось его не удалось найти
func findPortName(instanceId *string) string {
	keyPath := fmt.Sprintf("SYSTEM\\CurrentControlSet\\Enum\\%s\\Device Parameters", *instanceId)
	fmt.Println(keyPath)
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	if err != nil {
		fmt.Println("Registry error:", err)
		return NOT_FOUND
	}
	portName, _, err := key.GetStringValue("PortName")
	//fmt.Println("PORT NAME", portName)
	if err == registry.ErrNotExist {
		fmt.Println("Port name doesn't exists")
		return NOT_FOUND
	}
	if err != nil {
		fmt.Println("Error on getting port name", err.Error())
		return NOT_FOUND
	}
	return portName
}

// true - если порт изменился, иначе false
func (board *BoardToFlash) updatePortName(ID string) bool {
	instanceId := getInstanceId(ID)
	// такого устройства нет
	if instanceId == nil {
		board.PortName = NOT_FOUND
		return true
	}
	if len(instanceId) > 1 {
		fmt.Printf("updatePortName: found more than one devices that are matched ID = %s\n", ID)
		return false
	}
	portName := findPortName(&ID)
	if board.getPort() != portName {
		board.setPort(portName)
		return true
	}
	return false
}
