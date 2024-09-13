//go:build darwin

package main

import (
	"os/exec"
	"strconv"

	"howett.net/plist"
)

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
func detectBoards(boardTemplates []BoardTemplate) map[string]*BoardFlashAndSerial {
	boards := make(map[string]*BoardFlashAndSerial)
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

/*
true - если порт изменился или не найден, иначе false

назначает порту значение NOT_FOUND, если не удалось найти порт

TODO: переделать интерфейс функции для всех платформ, сделать, чтобы функция возвращала error

TODO: обновление нескольких портов
*/
func (board *BoardFlashAndSerial) updatePortName(ID string) bool {
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

func IOREGport(plistArr []IOREG, ID string, board *BoardFlashAndSerial) (portName string, foundID bool) {
	for _, entry := range plistArr {
		if (entry.SerialNumber == "" && strconv.FormatInt(entry.SessionID, 10) == ID) || entry.SerialNumber == ID {
			detectedBoard := NewBoardToFlash(board.Type, NOT_FOUND)
			collectBoardInfo(entry, detectedBoard)
			if detectedBoard.getPort() == "" || detectedBoard.getPort() == NOT_FOUND {
				printLog("can't find port name!")
				detectedBoard.setPortSync(NOT_FOUND)
				return NOT_FOUND, true
			}
			return detectedBoard.getPort(), true
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

func IOREGscan(plistArr []IOREG, boardTemplates []BoardTemplate, boards map[string]*BoardFlashAndSerial) {
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
						if ID == "" || ID == "0" {
							printLog("can't find ID!")
							goto SKIP
						}
						if detectedBoard.getPort() == "" || detectedBoard.getPort() == NOT_FOUND {
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

func collectBoardInfo(reg IOREG, board *BoardFlashAndSerial) (sessionID int64) {
	if reg.SerialNumber != "" {
		board.SerialID = reg.SerialNumber
	}
	if reg.Port != "" {
		board.setPortSync(reg.Port)
	}
	if reg.SessionID != 0 {
		sessionID = reg.SessionID
	}
	for _, child := range reg.Children {
		res := collectBoardInfo(child, board)
		if res != 0 {
			sessionID = res
		}
	}
	return sessionID
}
