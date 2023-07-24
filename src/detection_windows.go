//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"

	"golang.org/x/sys/windows/registry"
)

// возвращает пути к устройствам, согласно заданному шаблону, если они есть, иначе возвращает пустой массив (nil)
func getInstanceId(path string) []string {
	cmd := fmt.Sprintf("Get-PnpDevice -PresentOnly -InstanceId '%s' | Select-Object -Property InstanceId", path)
	cmdExec := exec.Command("powershell", cmd)
	cmdResult, err := cmdExec.CombinedOutput()
	var possiblePathes []string
	// значит такое устройство не подключено
	if err != nil {
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
		if v == 13 {
			continue
		}
		curStr += string(v)
	}
	return possiblePathes
}

// находит все подключённые платы, считает все найденные устройства за новые
func detectBoards() map[string]*BoardToFlash {
	vendors := vendorList()
	boardTypes := boardList()
	boards := make(map[string]*BoardToFlash)
	for _, vendor := range vendors {
		for _, boardType := range boardTypes[vendor] {
			cmdPathPattern := fmt.Sprintf("USB\\VID_%s&PID_%s*", vendor, boardType.ProductID)
			possiblePathes := getInstanceId(string(cmdPathPattern))
			// значит такое устройство не подключено
			if possiblePathes == nil {
				continue
			}
			// поиск портов, если удалось найти путь, то в список boards добавляется новое устройство
			for _, path := range possiblePathes {
				portName := findPortName(path)
				if portName == NOT_FOUND {
					continue
				}
				detectedBoard := BoardToFlash{
					boardType,
					portName,
				}
				boards[path] = &detectedBoard
			}
		}
	}
	return boards
}

// возвращает имя порта, либо константу NOT_FOUND, если не удалось его не удалось найти
func findPortName(instanceId string) string {
	keyPath := fmt.Sprintf("SYSTEM\\CurrentControlSet\\Enum\\%s\\Device Parameters", instanceId)
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
	portName := findPortName(instanceId[0])
	if board.PortName != portName {
		board.PortName = portName
		return true
	}
	return false
}
