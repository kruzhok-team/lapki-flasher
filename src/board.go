package main

import (
	"sync"

	"github.com/albenik/go-serial/v2"
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

func (board BoardType) hasBootloader() bool {
	return board.BootloaderTypeID > -1
}

type BoardFlashAndSerial struct {
	Type     BoardType
	PortName string
	SerialID string
	mu       sync.Mutex
	// устройство прошивается
	flashing bool
	// bootloader, связанный с платой, nil - если не найден, или отсутствует вообще
	refToBoot *BoardFlashAndSerial
	// монитор порта, nil значит, что монитор порта закрыт
	serialPortMonitor *serial.Port
	// канал для оповещения о том, что следует сменить бод
	serialMonitorChangeBaud chan int
	// клиент, который открыл монитор порта этого устройства
	serialMonitorClient *WebSocketConnection
	serialMonitorBaud   int
	serialMonitorOpen   bool
	serialMonitorWrite  chan string
}

func NewBoardToFlash(Type BoardType, PortName string) *BoardFlashAndSerial {
	var board BoardFlashAndSerial
	board.Type = Type
	board.PortName = PortName
	board.flashing = false

	if board.Type.hasBootloader() {
		var bootloader BoardFlashAndSerial
		board.refToBoot = &bootloader
		board.refToBoot.flashing = false
		bootTemplate := findTemplateByID(board.Type.BootloaderTypeID)
		if bootTemplate == nil {
			//TODO
		}
		board.refToBoot.Type = BoardType{
			typeID:           bootTemplate.ID,
			Name:             bootTemplate.Name,
			Controller:       bootTemplate.Controller,
			Programmer:       bootTemplate.Programmer,
			BootloaderTypeID: bootTemplate.BootloaderID,
		}
	}
	return &board
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

// подключено ли устройство
func (board *BoardFlashAndSerial) IsConnected() bool {
	return board.PortName != NOT_FOUND
}

// подключено ли устройство
func (board *BoardFlashAndSerial) IsConnectedSync() bool {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.IsConnected()
}

// найдено ли устройство
func (board *BoardFlashAndSerial) IsIdentified() bool {
	return board.SerialID != ""
}

// найдено ли устройство
func (board *BoardFlashAndSerial) IsIdentifiedSync() bool {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.IsIdentified()
}

// true = устройство заблокировано для прошивки
func (board *BoardFlashAndSerial) IsFlashBlocked() bool {
	return board.flashing
}

// true = устройство заблокировано для прошивки
func (board *BoardFlashAndSerial) IsFlashBlockedSync() bool {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.IsFlashBlocked()
}

// true = заблокировать устройство, false = разблокировать устройство
func (board *BoardFlashAndSerial) SetLock(lock bool) {
	board.flashing = lock
}

// true = заблокировать устройство, false = разблокировать устройство
func (board *BoardFlashAndSerial) SetLockSync(lock bool) {
	board.mu.Lock()
	defer board.mu.Unlock()
	board.SetLock(lock)
}

func (board *BoardFlashAndSerial) getPort() string {
	return board.PortName
}
func (board *BoardFlashAndSerial) getPortSync() string {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.getPort()
}

func (board *BoardFlashAndSerial) setPort(newPortName string) {
	board.PortName = newPortName
}
func (board *BoardFlashAndSerial) setPortSync(newPortName string) {
	board.mu.Lock()
	defer board.mu.Unlock()
	board.setPort(newPortName)
}

func (board *BoardFlashAndSerial) setSerialPortMonitor(serialPort *serial.Port, serialClient *WebSocketConnection, baud int) {
	board.serialPortMonitor = serialPort
	board.serialMonitorClient = serialClient
	board.serialMonitorChangeBaud = make(chan int)
	board.serialMonitorBaud = baud
	board.serialMonitorOpen = true
	board.serialMonitorWrite = make(chan string)
}
func (board *BoardFlashAndSerial) setSerialPortMonitorSync(serialPort *serial.Port, serialClient *WebSocketConnection, baud int) {
	board.mu.Lock()
	defer board.mu.Unlock()
	board.setSerialPortMonitor(serialPort, serialClient, baud)
}

func (board *BoardFlashAndSerial) isSerialMonitorOpen() bool {
	return board.serialPortMonitor != nil && board.serialMonitorOpen
}
func (board *BoardFlashAndSerial) isSerialMonitorOpenSync() bool {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.isSerialMonitorOpen()
}

func (board *BoardFlashAndSerial) closeSerialMonitor() {
	if board.serialPortMonitor == nil {
		return
	}
	if err := board.serialPortMonitor.Close(); err != nil {
		printLog(err.Error())
	}
	board.serialMonitorOpen = false
}
func (board *BoardFlashAndSerial) closeSerialMonitorSync() {
	board.mu.Lock()
	defer board.mu.Unlock()
	board.closeSerialMonitor()
}

func (board *BoardFlashAndSerial) getSerialMonitor() *serial.Port {
	return board.serialPortMonitor
}
func (board *BoardFlashAndSerial) getSerialMonitorSync() *serial.Port {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.getSerialMonitor()
}

func (d *Detector) boardExists(deviceID string) bool {
	_, exists := d.boards[deviceID]
	return exists
}
func (d *Detector) boardExistsSync(deviceID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.boardExists(deviceID)
}

// получить клиента, который занял монитор порта
func (board *BoardFlashAndSerial) getSerialMonitorClient() *WebSocketConnection {
	return board.serialMonitorClient
}

// получить клиента, который занял монитор порта
func (board *BoardFlashAndSerial) getSerialMonitorClientSync() *WebSocketConnection {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.getSerialMonitorClient()
}

func (board *BoardFlashAndSerial) getBaud() int {
	return board.serialMonitorBaud
}
func (board *BoardFlashAndSerial) getBaudSync() int {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.getBaud()
}
