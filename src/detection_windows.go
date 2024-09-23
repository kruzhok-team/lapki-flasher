//go:build windows
// +build windows

package main

import (
	"fmt"
	"log"
	"os/exec"
	"sort"

	"strings"

	"golang.org/x/sys/windows/registry"
)

type ArduinoOS struct {
	pathToDevice string
}

type MS1OS struct {
	pathesToDevices []string
}

// настройка ОС (для Windows она не требуется, но она здесь присутствует, чтобы обеспечить совместимость с другими платформами, которые использует свои реализации этой функции)
func setupOS() {

}

func getAllRegistryValues(path string) ([]string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	defer func() {
		if err := key.Close(); err != nil {
			printLog("Error on closing registry key:", err.Error())
		}
	}()
	if err != nil {
		return nil, err
	}
	registryValues, err := key.ReadValueNames(0)
	if err != nil {
		return nil, err
	}
	return registryValues, nil
}
func handleCloseRegistryKey(key registry.Key, path string) {
	if err := key.Close(); err != nil {
		printLog(fmt.Sprintf("Error on closing registry key in %s: %s", path, err.Error()))
	}
}
func getRegistryValues(path string) (registry.Key, []string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err != nil {
		printLog(fmt.Sprintf("Can't open %s registry key. %s", path, err.Error()))
		return key, nil, err
	}
	registryValues, err := key.ReadValueNames(0)
	if err != nil {
		printLog(fmt.Sprintf("Can't read values names in %s. %s", path, err.Error()))
		return key, nil, err
	}
	return key, registryValues, nil
}

/*
Возвращает пути к подключенным устройствам.

Если substring - не пустая строка, то возращает пути к устройствам, которые содержат substring.

Если ничего не удалось найти, то возвращает nil
*/
func getInstanceId(substring string) []string {
	// для сравнения строк в одном регистре
	substringUP := strings.ToUpper(substring)
	// получаем список, подключенных COM-портов
	const SERIALCOMM_PATH = "HARDWARE\\DEVICEMAP\\SERIALCOMM"
	serialCommKey, serialCommRegistryValues, err := getRegistryValues(SERIALCOMM_PATH)
	defer handleCloseRegistryKey(serialCommKey, SERIALCOMM_PATH)
	if err != nil {
		return nil
	}
	numOfActivePorts := len(serialCommRegistryValues)
	var currentPorts = make([]string, numOfActivePorts)
	for i, valueName := range serialCommRegistryValues {
		value, _, err := serialCommKey.GetStringValue(valueName)
		if err != nil {
			if err == registry.ErrUnexpectedType {
				continue
			}
			printLog(fmt.Sprintf("Error on getting registry values in %s. %s", SERIALCOMM_PATH, err.Error()))
			continue
		}
		currentPorts[i] = value
	}

	// получаем пути к устройствам, соотносим их со списком активным COM портов
	const DEVICE_PATHES = "SYSTEM\\CurrentControlSet\\Control\\COM Name Arbiter\\Devices"
	deviceKey, deviceRegistryValues, err := getRegistryValues(DEVICE_PATHES)
	defer handleCloseRegistryKey(deviceKey, DEVICE_PATHES)
	if err != nil {
		return nil
	}
	deviceRegistryValuesLen := len(deviceRegistryValues)
	sort.Strings(currentPorts)
	sort.Strings(deviceRegistryValues)
	currentPortsIndex := 0
	deviceRegistryValuesIndex := 0
	var pathesToDevices []string
	for currentPortsIndex < numOfActivePorts && deviceRegistryValuesIndex < deviceRegistryValuesLen {
		if currentPorts[currentPortsIndex] < deviceRegistryValues[deviceRegistryValuesIndex] {
			currentPortsIndex++
			continue
		}
		if currentPorts[currentPortsIndex] > deviceRegistryValues[deviceRegistryValuesIndex] {
			deviceRegistryValuesIndex++
			continue
		}
		if currentPorts[currentPortsIndex] == deviceRegistryValues[deviceRegistryValuesIndex] {
			pathToDevice, _, err := deviceKey.GetStringValue(currentPorts[currentPortsIndex])
			currentPortsIndex++
			deviceRegistryValuesIndex++
			if err != nil {
				if err == registry.ErrUnexpectedType {
					continue
				}
				printLog(fmt.Sprintf("Error on getting registry values in %s. %s", DEVICE_PATHES, err.Error()))
				continue
			}
			// здесь идёт преобразование переменной в путь к устройству
			pathToDevice = strings.ToUpper(pathToDevice)
			startIndex := strings.Index(pathToDevice, "USB")
			endIndex := strings.Index(pathToDevice, "#{")
			if startIndex < 0 || endIndex < 0 {
				printLog("Error, can't parse path!")
				continue
			}
			pathToDevice = pathToDevice[startIndex:endIndex]
			pathToDevice = strings.ReplaceAll(pathToDevice, "#", "\\")
			if substring == "" || strings.Contains(pathToDevice, substringUP) {
				pathesToDevices = append(pathesToDevices, pathToDevice)
			}
		}
	}
	return pathesToDevices
}

// находит все подключённые платы
func detectBoards(boardTemplates []BoardTemplate) map[string]*BoardFlashAndSerial {
	//startTime := time.Now()
	boards := make(map[string]*BoardFlashAndSerial)
	presentUSBDevices := getInstanceId("")
	// нет usb-устройств
	if presentUSBDevices == nil {
		return nil
	}
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
							typeID:           boardTemplate.ID,
							ProductID:        productID,
							VendorID:         vendorID,
							Name:             boardTemplate.Name,
							Controller:       boardTemplate.Controller,
							Programmer:       boardTemplate.Programmer,
							BootloaderTypeID: boardTemplate.BootloaderID,
							IsMSDevice:       boardTemplate.IsMSDevice,
						}
						detectedBoard := NewBoardToFlash(boardType, portName)
						serialIndex := strings.LastIndex(device, "\\")
						possibleSerialID := device[serialIndex+1:]
						if !strings.Contains(possibleSerialID, "&") {
							detectedBoard.SerialID = device[serialIndex+1:]
						}
						boards[device] = detectedBoard
						printLog("Device was found:", detectedBoard, device)
					}
				}
			}
		}
	}
	// windows распознает один МС-ТЮК как 4 разных устройства, поэтому их нужно отфильтровать
	composeMS(boards)
	//endTime := time.Now()
	//printLog("Detection time: ", endTime.Sub(startTime))
	printLog(boards)
	return boards
}

/*
возвращает имя порта, либо константу NOT_FOUND, если не удалось его не удалось найти

TODO: переделать интерфейс функции для всех платформ, сделать, чтобы функция возвращала error

TODO: обновление нескольких портов
*/
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
func (board *BoardFlashAndSerial) updatePortName(ID string) bool {
	// TODO: сделать проверку для МС-ТЮК
	if board.isMSDevice() {
		return false
	}
	instanceId := getInstanceId(ID)
	// такого устройства нет
	if instanceId == nil {
		board.setPort(NOT_FOUND)
		return true
	}
	if len(instanceId) > 1 {
		log.Printf("updatePortName: found more than one devices that are matched ID = %s\n", ID)
		return false
	}
	portName := findPortName(&ID)
	if board.getPortSync() != portName {
		board.setPortSync(portName)
		return true
	}
	return false
}

// перезагрузка порта
func rebootPort(portName string) (err error) {
	cmd := exec.Command("MODE", portName, "BAUD=1200")
	_, err = cmd.CombinedOutput()
	if err != nil {
		printLog(cmd.Args, err)
	}
	return err
}

// собрать части МС-ТЮК в одно устройство
func composeMS(boards map[string]*BoardFlashAndSerial) {
	type BoardID struct {
		ID           string
		friendlyName string
		Board        *BoardFlashAndSerial
	}
	var MSdevices []BoardID
	for boardID, board := range boards {
		if board.isMSDevice() {
			keyPath := fmt.Sprintf("SYSTEM\\CurrentControlSet\\Enum\\%s", boardID)
			key, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
			if err != nil {
				printLog("can't open key for ms1-device.", err.Error())
				return
			}
			defer func() {
				err := key.Close()
				if err != nil {
					printLog("windows: in function composeMS.", err.Error())
				}
			}()
			friendlyName, _, err := key.GetStringValue("FriendlyName")
			if err != nil {
				printLog("can't get FriendlyName property for ms1-device.", err.Error())
				return
			}
			MSdevices = append(MSdevices, BoardID{
				ID:           boardID,
				friendlyName: friendlyName,
				Board:        board,
			})
		}
	}
	sort.Slice(MSdevices, func(i, j int) bool {
		port1 := MSdevices[i].Board.getPort()
		port2 := MSdevices[j].Board.getPort()
		len1 := len(port1)
		len2 := len(port2)
		if len1 == len2 {
			return port1 < port2
		}
		return len1 < len2
	})
	for i, v := range MSdevices {
		if i%4 != 0 {
			MSdevices[i-i%4].Board.addPort(v.Board.getPort())
			delete(boards, v.ID)
		}
	}
	for index, value := range MSdevices {
		if index%4 != 0 {
			continue
		}
		indexMap := make(map[string]int, 4)
		var stringKeys []string
		for i := range 4 {
			indexMap[MSdevices[index+i].friendlyName] = index + i
			stringKeys = append(stringKeys, MSdevices[index+i].friendlyName)
		}
		sort.Strings(stringKeys)
		var orderedPortNames []string
		for i := range value.Board.PortNames {
			orderedPortNames = append(orderedPortNames, MSdevices[indexMap[stringKeys[i]]].Board.getPort())
		}
		value.Board.PortNames = orderedPortNames
	}
}

func (board *Arduino) Update() bool {
	return false
}

func (board *MS1) Update() bool {
	return false
}
