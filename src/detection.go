package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const NOT_FOUND = ""

type BoardType struct {
	ProductID      string
	VendorID       string
	Name           string
	Controller     string
	Programmer     string
	BootloaderName string
	BootloaderID   string
}

type BoardTemplate struct {
	VendorIDs      []string `json:"vendorIDs"`
	ProductIDs     []string `json:"productIDs"`
	Name           string   `json:"name"`
	Controller     string   `json:"controller"`
	Programmer     string   `json:"programmer"`
	BootloaderName string   `json:"bootloaderName"`
	BootloaderIDs  []string `json:"bootloaderIDs"`
}

func (board BoardType) hasBootloader() bool {
	return board.BootloaderID != ""
}

type BoardToFlash struct {
	Type     BoardType
	PortName string
	SerialID string
	mu       sync.Mutex
	// устройство прошивается
	flashing bool
}

func NewBoardToFlash(Type BoardType, PortName string) *BoardToFlash {
	var board BoardToFlash
	board.Type = Type
	board.PortName = PortName
	board.flashing = false
	return &board
}

// подключено ли устройство
func (board *BoardToFlash) IsConnected() bool {
	return board.PortName != NOT_FOUND
}

type DetectedBoard struct {
	FlashBoard BoardToFlash
	// true - устройство добавилсоь при последнем вызове функции update(), иначе если оно добавилось раньше false,
	// то есть устройства со значением true меняют своё значение на false при следующем вызове update()
	Status bool
}

type Detector struct {
	// список доступных для прошивки устройств
	boards map[string]*BoardToFlash
	mu     sync.Mutex
}

func NewDetector() *Detector {
	var d Detector
	d.boards = make(map[string]*BoardToFlash)
	return &d
}

// возвращает устройство, соответствующее ID, существует ли устройство в списке
func (d *Detector) GetBoard(ID string) (*BoardToFlash, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	value, exists := d.boards[ID]
	return value, exists
}

func (d *Detector) AddBoard(ID string, board *BoardToFlash) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.boards[ID] = board
}

// удаляет устройство из списка
func (d *Detector) DeleteBoard(ID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.boards, ID)
}

// удаляет все платы, которые не подключены в данный момент, возвращает ID устройств, которые были удалены
func (d *Detector) DeleteUnused() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var deletedID []string
	for ID, board := range d.boards {
		if !board.IsConnected() {
			deletedID = append(deletedID, ID)
			delete(d.boards, ID)
		}
	}
	return deletedID
}

func (d *Detector) GetBoards() ([]string, []*BoardToFlash) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var IDs []string
	var boards []*BoardToFlash
	for ID, board := range d.boards {
		IDs = append(IDs, ID)
		boards = append(boards, board)
	}
	return IDs, boards
}

func (d *Detector) GetIDs() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	IDs := make([]string, len(d.boards))
	index := 0
	for id := range d.boards {
		IDs[index] = id
		index++
	}
	return IDs
}

func (d *Detector) Update() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.boards == nil {
		d.boards = detectBoards()
		return
	}

	for ID, board := range d.boards {
		board.updatePortName(ID)
	}
	// сравниваем старый список с новым, чтобы найти новые устройства
	curBoards := detectBoards()
	for ID, value := range curBoards {
		// обращаемся напрямую к map, а не к функции GetBoard(), чтобы не обновлять состояние портов во второй раз
		_, exists := d.boards[ID]
		if !exists {
			d.boards[ID] = value
		}
	}
}

// получить количество, подключённых плат
func (d *Detector) boardsNum() int {
	return len(d.boards)
}

// удаляем устройства, которых больше нет и уведомляем об этом всех клиентов
func (d *Detector) DeleteAndAlert(newBoards map[string]*BoardToFlash, c *WebSocketConnection) {
	d.mu.Lock()
	for deviceID := range detector.boards {
		_, exists := newBoards[deviceID]
		if !exists {
			delete(detector.boards, deviceID)
			DeviceUpdateDelete(deviceID, c)
		}
	}
	d.mu.Unlock()
}

// true = устройство заблокировано для прошивки
func (board *BoardToFlash) IsFlashBlocked() bool {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.flashing
}

// true = заблокировать устройство, false = разблокировать устройство
func (board *BoardToFlash) SetLock(lock bool) {
	board.mu.Lock()
	defer board.mu.Unlock()
	board.flashing = lock
}

func (board *BoardToFlash) getPort() string {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.PortName
}

func (board *BoardToFlash) setPort(newPortName string) {
	board.mu.Lock()
	defer board.mu.Unlock()
	board.PortName = newPortName
}

//go:embed device_list.JSON
var boardTemplatesRaw []byte

func boardList() []BoardTemplate {
	start := time.Now()
	defer fmt.Println(time.Now().Sub(start))
	var result []BoardTemplate
	json.Unmarshal(boardTemplatesRaw, &result)
	return result
}
