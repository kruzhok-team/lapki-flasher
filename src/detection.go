package main

import (
	"strings"
)

const NOT_FOUND = ""

type BoardType struct {
	ProductID    string
	Name         string
	Controller   string
	Programmer   string
	Bootloader   string
	BootloaderID string
}

func (board BoardType) hasBootloader() bool {
	return board.BootloaderID != ""
}

type BoardToFlash struct {
	Type     BoardType
	PortName string
	// true - устройство добавилсоь при последнем вызове функции update(), иначе если оно добавилось раньше false,
	// то есть устройства со значением true меняют своё значение на false при следующем вызове update()
	Status bool
}

// подключено ли устройство
func (board BoardToFlash) IsConnected() bool {
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
}

func New() Detector {
	var d Detector
	d.boards = make(map[string]*BoardToFlash)
	return d
}

func (d *Detector) IsNew(ID string) bool {
	value, exists := d.boards[ID]
	if !exists {
		return false
	}
	return value.Status
}

func (d *Detector) setNew(ID string, status bool) {
	value, exists := d.boards[ID]
	if !exists {
		return
	}
	value.Status = status
}

func (d *Detector) GetBoard(ID string) (*BoardToFlash, bool, bool) {
	value, exists := d.boards[ID]
	portUpdated := false
	if exists {
		portUpdated = value.updatePortName(ID)
	}
	return value, exists, portUpdated
}

// удаляет все платы, которые не подключены в данный момент, возвращает ID устройств, которые были удалены
func (d *Detector) DeleteUnused() []string {
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
	var IDs []string
	var boards []*BoardToFlash
	for ID, board := range d.boards {
		IDs = append(IDs, ID)
		boards = append(boards, board)
	}
	return IDs, boards
}

func (d *Detector) GetNewBoards() ([]string, []*BoardToFlash) {
	var IDs []string
	var boards []*BoardToFlash
	for ID, board := range d.boards {
		if board.Status {
			IDs = append(IDs, ID)
			boards = append(boards, board)
		}
	}
	return IDs, boards
}

func (d *Detector) Update() {
	if d.boards == nil {
		d.boards = detectBoards()
		d.setNewStatusForAll(true)
		return
	}

	d.setNewStatusForAll(false)
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
			d.setNew(ID, true)
		}
	}
}

func (d *Detector) setNewStatusForAll(isNew bool) {
	for i := range d.boards {
		d.setNew(i, isNew)
	}
}

func vendorList() []string {
	// lower-case only
	vendors := []string{
		"2a03",
		"2341",
	}
	return vendors
}

func boardList() map[string][]BoardType {
	boardGroups := make(map[string][]string)
	boardGroups["2341,2a03"] = []string{
		"8037;Arduino Micro;ATmega32U4;avr109;Arduino Micro (bootloader);0037",
		"0043;Arduino Uno;ATmega328P;arduino;;",
	}
	vendorGroups := make(map[string][]BoardType)
	for vendorsStr, boardsStr := range boardGroups {
		var boards []BoardType
		for _, boardParams := range boardsStr {
			params := strings.Split(boardParams, ";")
			var board BoardType
			board.ProductID = params[0]
			board.Name = params[1]
			board.Controller = params[2]
			board.Programmer = params[3]
			board.Bootloader = params[4]
			board.BootloaderID = params[5]
			boards = append(boards, board)
		}
		vendorSep := strings.Split(vendorsStr, ",")
		for _, vendor := range vendorSep {
			vendorGroups[vendor] = boards
		}
	}
	return vendorGroups
}
