package main

import (
	"container/list"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
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
	BootloaderID int      `json:"bootloaderID"`
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
	// true = открыт монитор порта
	serialMonitorOpen bool
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

// найдено ли устройство
func (board *BoardFlashAndSerial) IsIdentified() bool {
	return board.SerialID != ""
}

type Detector struct {
	// список доступных для прошивки устройств
	boards         map[string]*BoardFlashAndSerial
	boardTemplates []BoardTemplate
	mu             sync.Mutex

	// симуляция плат
	fakeBoards map[string]*BoardFlashAndSerial

	// Список ID типов плат, которые не нужно добавлять, при обновлении.
	// Старые устройства, если они не отсоединялись, останутся в списке, даже если их typeID находится в списке
	dontAddTypes map[int]void

	boardActions *list.List
}

//go:embed device_list.JSON
var boardTemplatesRaw []byte

func NewDetector() *Detector {
	var d Detector
	d.boards = make(map[string]*BoardFlashAndSerial)
	// добавление фальшивых плат
	d.generateFakeBoards()
	json.Unmarshal(boardTemplatesRaw, &d.boardTemplates)
	d.dontAddTypes = make(map[int]void)
	d.boardActions = list.New()
	return &d
}

// Обновление текущего списка устройств.
// Вовращает:
// detectedBoards - все платы, которые удалось обнаружить;
// notAddedDevices - список новых устройств, которые были обнаружены, но не были добавлены, так как их типы были добавлены в исключения dontAddTypes;
// devicesInList - текущий список плат, без учёта notAddedDevices
func (d *Detector) Update() (
	detectedBoards map[string]*BoardFlashAndSerial,
	notAddedDevices map[string]*BoardFlashAndSerial,
	devicesInList map[string]*BoardFlashAndSerial) {

	d.mu.Lock()
	defer d.mu.Unlock()

	detectedBoards = detectBoards(d.boardTemplates)

	// добавление фальшивых плат к действительно обнаруженным
	if fakeBoardsNum > 0 {
		if detectedBoards == nil {
			detectedBoards = make(map[string]*BoardFlashAndSerial)
		}
		for ID, board := range d.fakeBoards {
			detectedBoards[ID] = board
		}
	}

	// обновление информации о старых устройствах и добавление новых

	notAddedDevices = make(map[string]*BoardFlashAndSerial)

	for deviceID, newBoard := range detectedBoards {
		oldBoard, exists := d.boards[deviceID]
		if exists {
			if oldBoard.getPortSync() != newBoard.PortName {
				oldBoard.setPortSync(newBoard.PortName)
				d.boardActions.PushBack(ActionWithBoard{board: oldBoard, boardID: deviceID, action: PORT_UPDATE})
			}
		} else {
			if _, ok := d.dontAddTypes[newBoard.Type.typeID]; ok {
				notAddedDevices[deviceID] = newBoard
			} else {
				d.boards[deviceID] = newBoard
				d.boardActions.PushBack(ActionWithBoard{board: newBoard, boardID: deviceID, action: ADD})
			}
		}
	}

	// удаление
	for deviceID := range d.boards {
		board, exists := detectedBoards[deviceID]
		if !exists {
			d.boardActions.PushBack(ActionWithBoard{board: board, boardID: deviceID, action: DELETE})
			delete(d.boards, deviceID)
		}
	}
	devicesInList = d.boards
	return
}

// Возвращает и удаляет первое действие в очереди.
// Если список действий пуст, то возвращает пустое действие и ложь в качестве второй переменной.
// Иначе вовращает действие с платой и истину.
func (d *Detector) PopFrontActionSync() (action ActionWithBoard, exists bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.boardActions.Len() > 0 {
		return d.boardActions.Remove(d.boardActions.Front()).(ActionWithBoard), true
	} else {
		return ActionWithBoard{}, false
	}
}

// попросить дектектора, чтобы он не добавлял новые устройства с данным typeID в список (старые устройства с этим typeID останутся в списке)
func (d *Detector) DontAddThisType(typeID int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dontAddTypes[typeID] = void{}
}

func (d *Detector) AddThisType(typeID int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.dontAddTypes, typeID)
}

// возвращает устройство, соответствующее ID, существует ли устройство в списке
func (d *Detector) GetBoardSync(ID string) (*BoardFlashAndSerial, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	value, exists := d.boards[ID]
	return value, exists
}

func (d *Detector) AddBoardSync(ID string, board *BoardFlashAndSerial) {
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
func (d *Detector) DeleteAndAlert(newBoards map[string]*BoardFlashAndSerial, c *WebSocketConnection) {
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
func (board *BoardFlashAndSerial) IsFlashBlocked() bool {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.flashing
}

// true = заблокировать устройство, false = разблокировать устройство
func (board *BoardFlashAndSerial) SetLock(lock bool) {
	board.mu.Lock()
	defer board.mu.Unlock()
	board.flashing = lock
}

func (board *BoardFlashAndSerial) getPortSync() string {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.PortName
}

func (board *BoardFlashAndSerial) setPortSync(newPortName string) {
	board.mu.Lock()
	defer board.mu.Unlock()
	board.PortName = newPortName
}

func (d *Detector) boardList() []BoardTemplate {
	return d.boardTemplates
}

// генерация фальшивых плат, которые будут восприниматься программой как настоящие
func (d *Detector) generateFakeBoards() {
	d.fakeBoards = make(map[string]*BoardFlashAndSerial)

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

func (board *BoardFlashAndSerial) setSerialMonitor(isOpen bool) {
	board.mu.Lock()
	defer board.mu.Unlock()
	board.serialMonitorOpen = isOpen
}

func (board *BoardFlashAndSerial) isSerialMonitorOpen() bool {
	board.mu.Lock()
	defer board.mu.Unlock()
	return board.serialMonitorOpen
}
