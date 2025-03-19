package main

import (
	"sync"
)

// список плат, которые распознаются загрузчиком, но не могут быть прошиты
var notSupportedBoards = []string{""}

type BoardType struct {
	typeID           int
	ProductID        string
	VendorID         string
	Name             string
	Controller       string
	Programmer       string
	BootloaderTypeID int
	IsMSDevice       bool
}

type BoardTemplate struct {
	ID           int      `json:"ID"`
	VendorIDs    []string `json:"vendorIDs"`
	ProductIDs   []string `json:"productIDs"`
	Name         string   `json:"name"`
	Controller   string   `json:"controller"`
	Programmer   string   `json:"programmer"`
	BootloaderID int      `json:"bootloaderID"`
	IsMSDevice   bool     `json:"isMSDevice"`
}

// является ли устройство МС-ТЮК
func (board BoardType) isMS() bool {
	return board.IsMSDevice
}

// является ли устройство Ардуино
func (board BoardType) isArduino() bool {
	return !board.IsMSDevice
}

func (board BoardType) hasBootloader() bool {
	return board.isArduino() && board.BootloaderTypeID > -1
}

type Board interface {
	IsConnected() bool
	GetSerialPort() string
	Flash(filePath string, logger chan any) (string, error)
	Update() bool
	GetWebMessageType() string
	GetWebMessage(name string, deviceID string) any
	Ping() error
	Reset() error
}

type Device struct {
	Name          string
	typeID        int
	Mu            sync.Mutex
	Flashing      bool
	Board         Board
	SerialMonitor SerialMonitor
}

func newDevice(name string, typeID int, board Board) *Device {
	device := Device{
		Name:     name,
		typeID:   typeID,
		Board:    board,
		Flashing: false,
		SerialMonitor: SerialMonitor{
			Open: false,
		},
	}
	return &device
}

// находит шаблон платы по его id
func findTemplateByID(boardID int) *BoardTemplate {
	var template BoardTemplate
	if boardID < len(detector.boardTemplates) {
		template = detector.boardTemplates[boardID]
		// ожидается, что в файле с шаблонами прошивок (device_list.JSON) нумеровка индексов будет идти по порядку, но если это не так, то придётся перебать все шаблоны
		if template.ID != boardID {
			foundCorrectBootloader := false
			for _, templ := range detector.boardTemplates {
				if templ.ID == boardID {
					template = templ
					foundCorrectBootloader = true
					break
				}
			}
			if foundCorrectBootloader {
				printLog("Не найден шаблон для устройства")
				return nil
			}
		}
	}
	return &template
}

// true = устройство заблокировано для прошивки
func (board *Device) IsFlashBlocked() bool {
	return board.Flashing
}

// true = устройство заблокировано для прошивки
func (board *Device) IsFlashBlockedSync() bool {
	board.Mu.Lock()
	defer board.Mu.Unlock()
	return board.IsFlashBlocked()
}

// true = заблокировать устройство, false = разблокировать устройство
func (dev *Device) SetLock(lock bool) {
	dev.Flashing = lock
}

// true = заблокировать устройство, false = разблокировать устройство
func (dev *Device) SetLockSync(lock bool) {
	dev.Mu.Lock()
	defer dev.Mu.Unlock()
	dev.SetLock(lock)
}

func (dev *Device) isSerialMonitorOpenSync() bool {
	dev.Mu.Lock()
	defer dev.Mu.Unlock()
	return dev.SerialMonitor.isOpen()
}

func (dev *Device) updateSync() bool {
	dev.Mu.Lock()
	defer dev.Mu.Unlock()
	return dev.Board.Update()
}

func (dev *Device) isConnectedSync() bool {
	dev.Mu.Lock()
	defer dev.Mu.Unlock()
	return dev.Board.IsConnected()
}

func (dev *Device) getSerialMonitorClientSync() *WebSocketConnection {
	dev.Mu.Lock()
	defer dev.Mu.Unlock()
	return dev.SerialMonitor.Client
}

func (dev *Device) getSerialMonitorBaudSync() int {
	dev.Mu.Lock()
	defer dev.Mu.Unlock()
	return dev.SerialMonitor.Baud
}

func (dev *Device) isFake() bool {
	switch dev.Board.(type) {
	case *FakeBoard:
		return true
	case *FakeMS:
		return true
	default:
		return false
	}
}
