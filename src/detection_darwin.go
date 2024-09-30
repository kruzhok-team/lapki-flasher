//go:build darwin

package main

import (
	"os/exec"
	"strconv"

	"howett.net/plist"
)

type ArduinoOS struct {
	ID string
}

type MS1OS struct {
}

type IOREG struct {
	VendorID  int64 `plist:"idVendor"`
	ProductID int64 `plist:"idProduct"`
	// Время с момента последнего запуска ОС.
	// Используется для идентификации устройства, в случае отсутствия серийного номера
	SessionID    int64   `plist:"sessionID"`
	SerialNumber string  `plist:"USB Serial Number"`
	Port         string  `plist:"IODialinDevice"`
	Children     []IOREG `plist:"IORegistryEntryChildren"`
}

// настройка ОС (для Darwin она не требуется, но она здесь присутствует, чтобы обеспечить совместимость с другими платформами, которые использует свои реализации этой функции)
func setupOS() {}

// находит все подключённые платы
// TODO: добавить поиск сериного номера
func detectBoards(boardTemplates []BoardTemplate) map[string]*Device {
	devices := make(map[string]*Device)
	cmd := exec.Command("ioreg", "-r", "-c", "IOUSBHostDevice", "-l", "-a")
	plistData, err := cmd.CombinedOutput()
	if err != nil {
		printLog("plist error", string(plistData), err.Error())
		return nil
	}
	plistArr := []IOREG{}
	format, err := plist.Unmarshal(plistData, &plistArr)
	if err != nil {
		printLog("unmarshal error:", err.Error(), cmd.String())
		printLog("plint format:", format)
		//printLog(string(plistData))
		return nil
	}
	IOREGscan(plistArr, boardTemplates, devices)
	return devices
}

func IOREGport(plistArr []IOREG, ID string, board *Arduino) (portName string, foundID bool) {
	for _, entry := range plistArr {
		if (entry.SerialNumber == "" && strconv.FormatInt(entry.SessionID, 10) == ID) || entry.SerialNumber == ID {
			detectedBoard := CopyArduino(board)
			detectedBoard.portName = NOT_FOUND
			collectArduinoBoardInfo(entry, detectedBoard)
			if detectedBoard.portName == NOT_FOUND {
				printLog("can't find port name!")
				detectedBoard.portName = NOT_FOUND
				return NOT_FOUND, true
			}
			return detectedBoard.portName, true
		}
	}
	for _, entry := range plistArr {
		port, found := IOREGport(entry.Children, ID, board)
		if found {
			return port, found
		}
	}
	return NOT_FOUND, false
}

// перезагрузка порта
func rebootPort(portName string) (err error) {
	// stty 115200 -F /dev/ttyUSB0 raw -echo
	cmd := exec.Command("stty", "-f", portName, "1200")
	_, err = cmd.CombinedOutput()
	printLog(cmd.Args)
	if err != nil {
		printLog(cmd.Args, err)
	}
	return err
}

func IOREGscan(plistArr []IOREG, boardTemplates []BoardTemplate, boards map[string]*Device) {
	for _, entry := range plistArr {
		isFound := false
		for _, boardTemplate := range boardTemplates {
			for _, productID := range boardTemplate.ProductIDs {
				PID, err := strconv.ParseInt(productID, 16, 64)
				if err != nil {
					printLog("conv of product id to decimal is failed: ", err.Error())
					continue
				}
				for _, vendorID := range boardTemplate.VendorIDs {
					VID, err := strconv.ParseInt(vendorID, 16, 64)
					if err != nil {
						printLog("conv of vendor id to decimal is failed: ", err.Error())
						continue
					}
					if entry.ProductID == PID && entry.VendorID == VID {
						if boardTemplate.IsMSDevice {
							// TODO
						} else {
							arduino := NewArduinoFromTemp(
								boardTemplate,
								NOT_FOUND,
								ArduinoOS{},
								NOT_FOUND,
							)
							ID := strconv.FormatInt(collectArduinoBoardInfo(entry, arduino), 10)
							if arduino.serialID != "" {
								ID = arduino.serialID
							}
							if ID == "" || ID == "0" {
								printLog("can't find ID!")
								goto SKIP
							}
							if arduino.portName == NOT_FOUND {
								printLog("can't find port name!")
								goto SKIP
							}
							arduino.ardOS.ID = ID
							detectedDevice := newDevice(
								boardTemplate.Name,
								boardTemplate.ID,
								arduino,
							)
							boards[ID] = detectedDevice
							printLog("Found device", ID, detectedDevice)
							isFound = true
							goto SKIP
						}
					}
				}
			}
		}
	SKIP:
		if !isFound {
			IOREGscan(entry.Children, boardTemplates, boards)
		}
	}
}

func collectArduinoBoardInfo(reg IOREG, board *Arduino) (sessionID int64) {
	if reg.SerialNumber != "" {
		board.serialID = reg.SerialNumber
	}
	if reg.Port != "" {
		board.portName = reg.Port
	}
	if reg.SessionID != 0 {
		sessionID = reg.SessionID
	}
	for _, child := range reg.Children {
		res := collectArduinoBoardInfo(child, board)
		if res != 0 {
			sessionID = res
		}
	}
	return sessionID
}

func (board *Arduino) Update() bool {
	cmd := exec.Command("ioreg", "-r", "-c", "IOUSBHostDevice", "-l", "-a")
	plistData, err := cmd.CombinedOutput()
	if err != nil {
		printLog("plist error", string(plistData), err.Error())
		board.portName = NOT_FOUND
		return true
	}
	plistArr := []IOREG{}
	format, err := plist.Unmarshal(plistData, &plistArr)
	if err != nil {
		printLog("unmarshal error:", err.Error(), cmd.String())
		printLog("plint format:", format)
		//printLog(string(plistData))
		board.portName = NOT_FOUND
		return true
	}
	portName, _ := IOREGport(plistArr, board.ardOS.ID, board)
	if portName == NOT_FOUND {
		board.portName = NOT_FOUND
		return true
	}
	if portName != board.portName {
		board.portName = portName
		return true
	}
	return false
}

func (board *MS1) Update() bool {
	return false
}
