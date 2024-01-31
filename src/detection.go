package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"golang.org/x/tools/go/analysis/passes/nilfunc"
)

const NOT_FOUND = ""

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
}

type BoardTemplate struct {
	ID           int      `json:"ID"`
	VendorIDs    []string `json:"vendorIDs"`
	ProductIDs   []string `json:"productIDs"`
	Name         string   `json:"name"`
	Controller   string   `json:"controller"`
	Programmer   string   `json:"programmer"`
	BootloaderID int      `json:"bootloaderIDs"`
}

func (board BoardType) hasBootloader() bool {
	return board.BootloaderTypeID > -1
}

type BoardToFlash struct {
	Type     BoardType
	PortName string
	SerialID string
	mu       sync.Mutex
	// устройство прошивается
	flashing bool
	// bootloader, связанный с платой, nil - если не найден, или отсутствует вообще
	refToBoot *BoardToFlash
}

func NewBoardToFlash(Type BoardType, PortName string) *BoardToFlash {
	var board BoardToFlash
	board.Type = Type
	board.PortName = PortName
	board.flashing = false

	if board.Type.hasBootloader() {
		var bootloader BoardToFlash
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
func (board *BoardToFlash) IsConnected() bool {
	return board.PortName != NOT_FOUND
}

// найдено ли устройство
func (board *BoardToFlash) IsIdentified() bool {
	return board.SerialID != ""
}

type DetectedBoard struct {
	FlashBoard BoardToFlash
	// true - устройство добавилсоь при последнем вызове функции update(), иначе если оно добавилось раньше false,
	// то есть устройства со значением true меняют своё значение на false при следующем вызове update()
	Status bool
}

type DeviceWithBootloader struct {
	device *BoardToFlash
	ID     string
}
func (dewboot *DeviceWithBootloader) {
	dewb
}
type Detector struct {
	// список доступных для прошивки устройств
	boards         map[string]*BoardToFlash
	boardTemplates []BoardTemplate
	mu             sync.Mutex

	// симуляция плат
	fakeBoards map[string]*BoardToFlash

	// устройство с bootloader, которое планируется прошить (загрузчик не может прошивать несколько устройств с bootloader одновременно)
	deviceWithBootloader DeviceWithBootloader
}

//go:embed device_list.JSON
var boardTemplatesRaw []byte

func NewDetector() *Detector {
	var d Detector
	d.boards = make(map[string]*BoardToFlash)
	// добавление фальшивых плат
	d.generateFakeBoards()
	json.Unmarshal(boardTemplatesRaw, &d.boardTemplates)
	d.deviceWithBootloader.device = nil
	return &d
}

// Обновление текущего списка устройств.
// Вовращает:
//
// updatedPort - список устройств с новыми значениями портов;
// newDevices - список устройств, которых не было в старом списке;
// deletedDevices - спискок устройств, которые были в старом списке, но которых нет в обновлённом;
func (d *Detector) Update() (detectedBoards map[string]*BoardToFlash, updatedPort map[string]*BoardToFlash, newDevices map[string]*BoardToFlash, deletedDevices map[string]*BoardToFlash) {
	detectedBoards = detectBoards()

	// добавление фальшивых плат к действительно обнаруженным
	if fakeBoardsNum > 0 {
		if detectedBoards == nil {
			detectedBoards = make(map[string]*BoardToFlash)
		}
		for ID, board := range detector.fakeBoards {
			detectedBoards[ID] = board
		}
	}

	// обновление информации о старых устройствах и добавление новых
	updatedPort = make(map[string]*BoardToFlash)
	newDevices = make(map[string]*BoardToFlash)
	deletedDevices = make(map[string]*BoardToFlash)
	if d.BootloaderDeviceFlash() {
		detectedBoards
	}
	bootloaderCnt := 0
	for deviceID, newBoard := range detectedBoards {
		oldBoard, exists := detector.GetBoard(deviceID)
		if exists {
			if oldBoard.getPort() != newBoard.PortName {
				oldBoard.setPort(newBoard.PortName)
				updatedPort[deviceID] = newBoard
			}
		} else {
			if d.isBootloaderFlashing() && d.deviceWithBootloader.Type.BootloaderTypeID == newBoard.Type.typeID {
				bootloaderCnt++
				d.deviceWithBootloader.refToBoot = newBoard
				if bootloaderCnt > 1 {
					printLog("Bootloader: ошибка")
				}
				continue
			}
			detector.AddBoard(deviceID, newBoard)
			newDevices[deviceID] = newBoard
		}
	}
	// удаление
	detector.mu.Lock()
	for deviceID := range detector.boards {
		board, exists := detectedBoards[deviceID]
		if !exists {
			deletedDevices[deviceID] = board
			delete(detector.boards, deviceID)
		}
	}
	detector.mu.Unlock()

	return
}

func (d *Detector) isBootloaderFlashing() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.deviceWithBootloader != nil
}

func (d *Detector) BootloaderDeviceFlash(board *BoardToFlash) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deviceWithBootloader = board
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

func (d *Detector) boardList() []BoardTemplate {
	return d.boardTemplates
}

// генерация фальшивых плат, которые будут восприниматься программой как настоящие
func (d *Detector) generateFakeBoards() {
	d.fakeBoards = make(map[string]*BoardToFlash)

	// фальшивые параметры для фальшивых плат
	id := -1
	vendorID := "-0000"
	productID := "-0000"
	name := "Fake Board"
	controller := "Fake Controller"
	programmer := "Fake Programmer"
	bootloaderID := -1
	fakeType := BoardType{
		typeID:           id,
		ProductID:        productID,
		VendorID:         vendorID,
		Name:             name,
		Controller:       controller,
		Programmer:       programmer,
		BootloaderTypeID: bootloaderID,
	}

	for i := 0; i < fakeBoardsNum; i++ {
		fakeID := fmt.Sprintf("fakeid-%d", i)
		fakePort := fmt.Sprintf("fakecom-%d", i)
		newFakeBoard := NewBoardToFlash(fakeType, fakePort)
		newFakeBoard.SerialID = fakeID
		d.fakeBoards[fakeID] = newFakeBoard
	}

	for ID, board := range d.fakeBoards {
		d.boards[ID] = board
	}
}

// true = плата с данным ID является фальшивой
func (d *Detector) isFake(ID string) bool {
	if fakeBoardsNum > 0 {
		_, exists := d.fakeBoards[ID]
		if exists {
			return true
		}
	}
	return false
}
