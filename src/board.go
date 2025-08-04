package main

import (
	"encoding/json"
	"sync"
)

// список плат, которые распознаются загрузчиком, но не могут быть прошиты
var notSupportedBoards = []string{""}

type PidVidType struct {
	ProductID string `json:"productID"`
	VendorID  string `json:"vendorID"`
}

type ArduinoPayload struct {
	Controller   string `json:"controller"`
	Programmer   string `json:"programmer"`
	BootloaderID int    `json:"bootloaderID"`
}

type BoardTemplate struct {
	ID                 int             `json:"ID"`
	PidVid             []PidVidType    `json:"pidvid"`
	Name               string          `json:"name"`
	Type               string          `json:"type"`
	TypePayload        json.RawMessage `json:"typePayload"`
	FlashFileExtension string          `json:"flashFileExtension"`
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
	GetMetaData() (any, error)
}

type Device struct {
	TypeDesc      *BoardTemplate
	Mu            sync.Mutex
	Flashing      bool
	Board         Board
	SerialMonitor SerialMonitor
}

func newDevice(typeDesc BoardTemplate, board Board) *Device {
	device := Device{
		TypeDesc: &typeDesc,
		Board:    board,
		Flashing: false,
		SerialMonitor: SerialMonitor{
			Open: false,
		},
	}
	return &device
}

func (temp *BoardTemplate) IsMSDevice() bool {
	return temp.Type == "tjc-ms"
}

func (temp *BoardTemplate) IsArduinoDevice() bool {
	return temp.Type == "arduino"
}

func (temp *BoardTemplate) IsBlgMbDevice() bool {
	return temp.Type == "blg-mb"
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
