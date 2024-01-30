//go:build windows
// +build windows

package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// настройка ОС (для Windows она не требуется, но она здесь присутствует, чтобы обеспечить совместимость с другими платформами, которые использует свои реализации этой функции)
func setupOS() {

}

// возвращает пути к устройствам, согласно заданному шаблону, если они есть, иначе возвращает nil
func getInstanceId(substring string) []string {
	//start := time.Now()
	keyPath := "SYSTEM\\CurrentControlSet\\Services\\usbser\\Enum"
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	defer func() {
		if err := key.Close(); err != nil {
			printLog("Error on closing registry key:", err.Error())
		}
	}()
	if err != nil {
		printLog("No devices are connected or drivers are not installed", err)
		return nil
	}
	registryValues, err := key.ReadValueNames(0)
	if err != nil {
		printLog("Error on getting registry value names:", err.Error())
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
			printLog("Error on getting registry values:", err.Error())
			continue
		}
		if strings.Contains(strings.ToLower(value), substring) {
			possiblePathes = append(possiblePathes, value)
		}
	}
	//printLog("get instance:", time.Now().Sub(start))
	return possiblePathes
}

// находит все подключённые платы
func detectBoards() map[string]*BoardToFlash {
	//startTime := time.Now()
	boards := make(map[string]*BoardToFlash)
	presentUSBDevices := getInstanceId("usb")
	// нет usb-устройств
	if presentUSBDevices == nil {
		return nil
	}
	boardTemplates := detector.boardList()
	//fmt.Println(boardTemplates)
	for _, line := range presentUSBDevices {
		device := strings.TrimSpace(line)
		deviceLen := len(device)
		for _, boardTemplate := range boardTemplates {
			for _, vendorID := range boardTemplate.VendorIDs {
				for _, productID := range boardTemplate.ProductIDs {
					pathPattern := fmt.Sprintf("USB\\VID_%s&PID_%s", vendorID, productID)
					pathLen := len(pathPattern)
					// нашли подходящее устройство
					//printLog(strings.ToLower(device[:pathLen]), strings.ToLower(pathPattern))
					if pathLen <= deviceLen && strings.EqualFold(device[:pathLen], pathPattern) {
						portName := findPortName(&device)
						if portName == NOT_FOUND {
							printLog(device)
							continue
						}
						boardType := BoardType{
							boardTemplate.ID,
							productID,
							vendorID,
							boardTemplate.Name,
							boardTemplate.Controller,
							boardTemplate.Programmer,
							boardTemplate.BootloaderID,
						}
						detectedBoard := NewBoardToFlash(boardType, portName)
						serialIndex := strings.LastIndex(device, "\\")
						possibleSerialID := device[serialIndex+1:]
						if !strings.Contains(possibleSerialID, "&") {
							detectedBoard.SerialID = device[serialIndex+1:]
						}
						boards[device] = detectedBoard
						printLog("Device was found:", detectedBoard)
					}
				}
			}
		}
	}
	//endTime := time.Now()
	//printLog("Detection time: ", endTime.Sub(startTime))
	return boards
}

// возвращает имя порта, либо константу NOT_FOUND, если не удалось его не удалось найти
func findPortName(instanceId *string) string {
	keyPath := fmt.Sprintf("SYSTEM\\CurrentControlSet\\Enum\\%s\\Device Parameters", *instanceId)
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	defer func() {
		if err := key.Close(); err != nil {
			printLog("Error on closing registry key:", err.Error())
		}
	}()
	if err != nil {
		printLog("Registry error:", err)
		return NOT_FOUND
	}
	portName, _, err := key.GetStringValue("PortName")
	//fmt.Println("PORT NAME", portName)
	if err == registry.ErrNotExist {
		printLog("Port name doesn't exists")
		return NOT_FOUND
	}
	if err != nil {
		printLog("Error on getting port name:", err.Error())
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
		log.Printf("updatePortName: found more than one devices that are matched ID = %s\n", ID)
		return false
	}
	portName := findPortName(&ID)
	if board.getPort() != portName {
		board.setPort(portName)
		return true
	}
	return false
}

// перезагрузка порта
func rebootPort(portName string) (err error) {
	cmd := exec.Command("MODE", portName, "BAUD=1200")
	_, err = cmd.CombinedOutput()
	print(cmd.Args, err)
	return err
}

// найти bootloader для композитного устройства такого как Arduino Micro
func refToBoot(board *BoardToFlash) (bootloader *BoardToFlash) {
	if board.refToBoot != nil {
		return board.refToBoot
	}

	return bootloader
}
