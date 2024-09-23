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
	Flash(filePath string) (string, error)
	Update() bool
}

type Device struct {
	Name          string
	ProductID     string
	VendorID      string
	Mu            sync.Mutex
	Flashing      bool
	Board         Board
	SerialMonitor SerialMonitor
}

type BoardFlashAndSerial struct {
	Type BoardType
	// список портов, для arduino-подобных устройств он состоит из одного элемента, для МС-ТЮК может состоять из нескольких
	PortNames []string
	SerialID  string
	mu        sync.Mutex
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
	// адрес устройства (для МС-ТЮК), он может не совпадать с реальным адресом МС-ТЮК, используется для запоминания и использования где-то ещё
	msAddress string
}

func newBoard(Type BoardType) *BoardFlashAndSerial {
	var board BoardFlashAndSerial
	board.Type = Type
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

func NewBoardToFlash(Type BoardType, PortName string) *BoardFlashAndSerial {
	board := newBoard(Type)
	board.setPort(PortName)
	return board
}

func NewBoardToFlashPorts(Type BoardType, PortNames []string) *BoardFlashAndSerial {
	board := newBoard(Type)
	board.setPorts(PortNames)
	return board
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
	return board.getPort() != NOT_FOUND
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

// получить первый в списке порт, возвращает константу NOT_FOUND, если массив пустой
func (board *BoardFlashAndSerial) getPort() string {
	if board.PortNames == nil {
		return NOT_FOUND
	}
	return board.PortNames[0]
}

// получить первый в списке порт, возвращает константу NOT_FOUND, если массив пустой
func (board *BoardFlashAndSerial) getPortSync() string {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.getPort()
}

// поменять первый в списке порт, если массив пустой, то создаёт новый массив, где единственным элементом является переданная строка
func (board *BoardFlashAndSerial) setPort(newPortName string) {
	if board.PortNames == nil {
		board.PortNames = make([]string, 1)
	}
	board.PortNames[0] = newPortName
}

// поменять первый в списке порт, если массив пустой, то создаёт новый массив, где единственным элементом является переданная строка
func (board *BoardFlashAndSerial) setPortSync(newPortName string) {
	board.mu.Lock()
	defer board.mu.Unlock()
	board.setPort(newPortName)
}

// получить список портов
func (board *BoardFlashAndSerial) getPorts() []string {
	return board.PortNames
}

// получить список портов
func (board *BoardFlashAndSerial) getPortsSync() []string {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.getPorts()
}

// поменять список портов, копирует ссылку на переданный массив!
func (board *BoardFlashAndSerial) setPorts(newPortNames []string) {
	board.PortNames = newPortNames
}

// поменять список портов, копирует ссылку на переданный массив!
func (board *BoardFlashAndSerial) setPortsSync(newPortNames []string) {
	board.mu.Lock()
	defer board.mu.Unlock()
	board.setPorts(newPortNames)
}

// добавить порт в список
func (board *BoardFlashAndSerial) addPort(newPortName string) {
	board.PortNames = append(board.PortNames, newPortName)
}

// количество портов
func (board *BoardFlashAndSerial) portsNum() int {
	if board.PortNames == nil {
		return 0
	}
	return len(board.PortNames)
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

func (board *BoardFlashAndSerial) isMSDevice() bool {
	return board.Type.IsMSDevice
}
func (board *BoardFlashAndSerial) isMSDeviceSync() bool {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.isMSDevice()
}

func (board *BoardFlashAndSerial) getSerialPortName() string {
	if board.isMSDevice() {
		return board.PortNames[3]
	} else {
		return board.getPort()
	}
}

// запомнить адрес устройства МС-ТЮК
func (board *BoardFlashAndSerial) setAddressMS(address string) {
	board.msAddress = address
}

// получить последнее записанное значение адреса МС-ТЮК
func (board *BoardFlashAndSerial) getAddressMS() string {
	return board.msAddress
}
