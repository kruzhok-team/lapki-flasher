//go:build darwin

package main

import (
	"os/exec"
	"strconv"

	"howett.net/plist"
)

type IOREG struct {
	VendorID     int64   `plist:"idVendor"`
	ProductID    int64   `plist:"idProduct"`
	LocationID   int64   `plist:"locationID"`
	SerialNumber string  `plist:"USB Serial Number"`
	Port         string  `plist:"IODialinDevice"`
	Children     []IOREG `plist:"IORegistryEntryChildren"`
}

// настройка ОС (для Darwin она не требуется, но она здесь присутствует, чтобы обеспечить совместимость с другими платформами, которые использует свои реализации этой функции)
func setupOS() {}

// находит все подключённые платы
// TODO: добавить поиск сериного номера
func detectBoards(boardTemplates []BoardTemplate) map[string]*BoardToFlash {
	boards := make(map[string]*BoardToFlash)
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
	IOREGscan(plistArr, boardTemplates, boards)
	return boards
}

// true - если порт изменился или не найден, иначе false
// назначает порту значение NOT_FOUND, если не удалось найти порт
// TODO: переделать интерфейс функции для всех платформ, сделать, чтобы функция возвращала error
func (board *BoardToFlash) updatePortName(ID string) bool {
	cmd := exec.Command("ioreg", "-r", "-c", "IOUSBHostDevice", "-l", "-a")
	plistData, err := cmd.CombinedOutput()
	if err != nil {
		printLog("plist error", string(plistData), err.Error())
		board.setPortSync(NOT_FOUND)
		return true
	}
	plistArr := []IOREG{}
	format, err := plist.Unmarshal(plistData, &plistArr)
	if err != nil {
		printLog("unmarshal error:", err.Error(), cmd.String())
		printLog("plint format:", format)
		//printLog(string(plistData))
		board.setPortSync(NOT_FOUND)
		return true
	}
	portName, _ := IOREGport(plistArr, ID, board)
	if portName == NOT_FOUND {
		board.setPortSync(NOT_FOUND)
		return true
	}
	if portName != board.getPortSync() {
		board.setPortSync(portName)
		return true
	}
	return false
}

func IOREGport(plistArr []IOREG, ID string, board *BoardToFlash) (portName string, foundID bool) {
	decimalVID, err := strconv.ParseInt(board.Type.VendorID, 16, 64)
	if err != nil {
		printLog(err.Error())
	}
	decimalPID, err := strconv.ParseInt(board.Type.ProductID, 16, 64)
	if err != nil {
		printLog(err.Error())
	}
	for _, entry := range plistArr {
		if strconv.FormatInt(entry.LocationID, 10) == ID || entry.SerialNumber == ID {
			if entry.ProductID != decimalPID || entry.VendorID != decimalVID {
				return NOT_FOUND, true
			}
			detectedBoard := NewBoardToFlash(board.Type, NOT_FOUND)
			collectBoardInfo(entry, detectedBoard)
			if detectedBoard.PortName == "" || detectedBoard.PortName == NOT_FOUND {
				printLog("can't find port name!")
				detectedBoard.setPortSync(NOT_FOUND)
				return NOT_FOUND, true
			}
			return detectedBoard.PortName, true
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

func IOREGscan(plistArr []IOREG, boardTemplates []BoardTemplate, boards map[string]*BoardToFlash) {
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
						boardType := BoardType{
							typeID:           boardTemplate.ID,
							ProductID:        productID,
							VendorID:         vendorID,
							Name:             boardTemplate.Name,
							Controller:       boardTemplate.Controller,
							Programmer:       boardTemplate.Programmer,
							BootloaderTypeID: boardTemplate.BootloaderID,
						}
						detectedBoard := NewBoardToFlash(boardType, NOT_FOUND)
						ID := strconv.FormatInt(collectBoardInfo(entry, detectedBoard), 10)
						if detectedBoard.SerialID != "" {
							ID = detectedBoard.SerialID
						}
						if detectedBoard.PortName == "" || detectedBoard.PortName == NOT_FOUND {
							printLog("can't find port name!")
							goto SKIP
						}
						boards[ID] = detectedBoard
						printLog("Found device", ID, detectedBoard)
						isFound = true
						goto SKIP
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

func collectBoardInfo(reg IOREG, board *BoardToFlash) (locationID int64) {
	if reg.SerialNumber != "" {
		board.SerialID = reg.SerialNumber
	}
	if reg.Port != "" {
		board.setPortSync(reg.Port)
	}
	if reg.LocationID != 0 {
		locationID = reg.LocationID
	}
	for _, child := range reg.Children {
		res := collectBoardInfo(child, board)
		if res != 0 {
			locationID = res
		}
	}
	return locationID
}
