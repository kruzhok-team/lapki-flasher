//go:build windows
// +build windows

package main

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

// возвращает пути к устройствам, согласно заданному шаблону, если они есть, иначе возвращает nil
func getInstanceId(substring string) []string {
	start := time.Now()
	keyPath := "SYSTEM\\CurrentControlSet\\Services\\usbser\\Enum"
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	defer key.Close()
	if err != nil {
		fmt.Println("Can't find devices, perhaps drivers are needed?", err)
		return nil
	}
	registryValues, err := key.ReadValueNames(0)
	if err != nil {
		fmt.Println("Error on getting registry value names:", err.Error())
		return nil
	}
	var possiblePathes []string
	substring = strings.ToLower(substring)
	for _, valueName := range registryValues {
		value, _, err := key.GetStringValue(valueName)
		if err != nil {
			if err == registry.ErrUnexpectedType {
				continue
			}
			fmt.Println("Error on getting registry values:", err.Error())
			continue
		}
		if strings.Contains(strings.ToLower(value), substring) {
			possiblePathes = append(possiblePathes, value)
		}
		fmt.Println(value)
	}
	fmt.Println("get instance:", time.Now().Sub(start))
	return possiblePathes
}

// находит все подключённые платы
func detectBoards() map[string]*BoardToFlash {
	startTime := time.Now()
	boards := make(map[string]*BoardToFlash)
	presentUSBDevices := getInstanceId("usb")
	// нет usb-устройств
	if presentUSBDevices == nil {
		return nil
	}
	boardTemplates := boardList()
	fmt.Println(boardTemplates)
	for _, line := range presentUSBDevices {
		device := strings.TrimSpace(line)
		deviceLen := len(device)
		for _, boardTemplate := range boardTemplates {
			for _, vendorID := range boardTemplate.VendorIDs {
				for _, productID := range boardTemplate.ProductIDs {
					pathPattern := fmt.Sprintf("USB\\VID_%s&PID_%s", vendorID, productID)
					pathLen := len(pathPattern)
					// нашли подходящее устройство
					fmt.Println(strings.ToLower(device[:pathLen]), strings.ToLower(pathPattern))
					if pathLen <= deviceLen && strings.ToLower(device[:pathLen]) == strings.ToLower(pathPattern) {
						portName := findPortName(&device)
						if portName == NOT_FOUND {
							fmt.Println(device)
							continue
						}
						boardType := BoardType{
							productID,
							vendorID,
							boardTemplate.Name,
							boardTemplate.Controller,
							boardTemplate.Programmer,
							boardTemplate.BootloaderName,
							"",
						}
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
	defer key.Close()
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
		fmt.Println("Error on getting port name:", err.Error())
		return NOT_FOUND
	}
	return portName
}

// true - если порт изменился или не найден, иначе false
// назначает порту значение NOT_FOUND, если не удалось найти порт
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
