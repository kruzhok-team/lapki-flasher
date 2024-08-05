// обработка и отправка сообщений
package main

import (
	"encoding/json"
	"log"
	"strings"
)

// обработчик события
type EventHandler func(event Event, c *WebSocketConnection) error

// Общий вид для всех сообщений, как от клиента так и от сервера
// (исключение: бинарные данные от клиента, но все равно они приводятся сервером к этой структуре)
type Event struct {
	// Тип сообщения (flash-start, get-list и т.д.)
	Type string `json:"type"`
	// Параметры сообщения, не все сообщения обязаны иметь параметры
	Payload json.RawMessage `json:"payload"`
}

type DeviceMessage struct {
	ID         string `json:"deviceID"`
	Name       string `json:"name,omitempty"`
	Controller string `json:"controller,omitempty"`
	Programmer string `json:"programmer,omitempty"`
	PortName   string `json:"portName,omitempty"`
	SerialID   string `json:"serialID,omitempty"`
}

type FlashStartMessage struct {
	ID       string `json:"deviceID"`
	FileSize int    `json:"fileSize"`
}

type FlashBlockMessage struct {
	BlockID int    `json:"blockID"`
	Data    []byte `json:"data"`
}

type FlashBlockMessageString struct {
	BlockID int    `json:"blockID"`
	Data    string `json:"data"`
}

type DeviceUpdateDeleteMessage struct {
	ID string `json:"deviceID"`
}

type DeviceUpdatePortMessage struct {
	ID       string `json:"deviceID"`
	PortName string `json:"portName"`
}

type MaxFileSizeMessage struct {
	Size int `json:"size"`
}

type SerialConnectMessage struct {
	ID   string `json:"deviceID"`
	Baud int    `json:"baud"`
}

type SerialConnectionStatusMessage struct {
	ID          string `json:"deviceID"`
	IsConnected bool   `json:"isConnected"`
	Comment     string `json:"comment"`
}

type SerialDisconnectMessage struct {
	ID string `json:"deviceID"`
}

type SerialSentStatusMessage struct {
	ID      string `json:"deviceID"`
	HasSent bool   `json:"hasSent"`
}

// тип данных для serial-device-read и serial-send
type SerialMessage struct {
	ID  string `json:"deviceID"`
	Msg string `json:"msg"`
}

// типы сообщений (событий)
const (
	// запрос на получение списка всех устройств
	GetListMsg = "get-list"
	// описание устройства
	DeviceMsg = "device"
	// запрос на прошивку устройства
	FlashStartMsg = "flash-start"
	// прошивка прошла успешна
	FlashDoneMsg = "flash-done"
	// запрос на следующий блок бинарных данных
	FlashNextBlockMsg = "flash-next-block"
	// сообщение, для отметки бинарных данных загружаемого файла прошивки, прикрепляется сервером к сообщению после получения данных бинарного типа
	FlashBinaryBlockMsg = "flash-block"
	// устройство удалено из списка
	DeviceUpdateDeleteMsg = "device-update-delete"
	// устройство поменяло порт
	DeviceUpdatePortMsg = "device-update-port"
	GetMaxFileSizeMsg   = "get-max-file-size"
	MaxFileSizeMsg      = "max-file-size"
	// устройства не найдены
	EmptyListMsg = "empty-list"
	// запрос на запуск монитора порта
	SerialConnectMsg = "serial-connect"
	// статус соединения с устройством (монитора порта)
	SerialConnectionStatusMsg = "serial-connection-status"
	// закрыть монитор порта
	SerialDisconnectMsg = "serial-disconnect"
	// запрос на отправку сообщения на устройство
	SerialSendMsg = "serial-send"
	// статус отправленного сообщения на устройство (удалось ли его отправить или нет)
	SerialSentStatusMsg = "serial-sent-status"
	// сообщение от устройства
	SerialDeviceReadMsg = "serial-device-read"
)

// отправить клиенту список всех устройств
func GetList(event Event, c *WebSocketConnection) error {
	printLog("get-list")
	if c.getListCooldown.isBlocked() {
		return ErrGetListCoolDown
	}
	UpdateList(c, nil)
	if detector.boardsNum() == 0 {
		c.sendOutgoingEventMessage(EmptyListMsg, nil, false)
	}
	return nil
}

// отправить клиенту описание устройства
// lastGetListDevice - дополнительная переменная, берётся только первое значение, остальные будут игнорироваться
func Device(deviceID string, board *BoardToFlash, toAll bool, c *WebSocketConnection) error {
	//printLog("device")
	boardMessage := DeviceMessage{
		deviceID,
		board.Type.Name,
		board.Type.Controller,
		board.Type.Programmer,
		board.PortName,
		board.SerialID,
	}
	err := c.sendOutgoingEventMessage(DeviceMsg, boardMessage, toAll)
	if err != nil {
		printLog("device() error:", err.Error())
	}
	return err
}

// сообщение о том, что порт обновлён
func DeviceUpdatePort(deviceID string, board *BoardToFlash, c *WebSocketConnection) {
	c.sendOutgoingEventMessage(DeviceUpdatePortMsg, newDeviceUpdatePortMessage(board, deviceID), true)
}

// сообщение о том, что устройство удалено
func DeviceUpdateDelete(deviceID string, c *WebSocketConnection) {
	c.sendOutgoingEventMessage(DeviceUpdateDeleteMsg, newDeviceUpdateDeleteMessage(deviceID), true)
}

// подготовка к чтению файла с прошивкой и к его загрузке на устройство
func FlashStart(event Event, c *WebSocketConnection) error {
	log.Println("Flash-start")
	if c.IsFlashing() {
		return ErrFlashNotFinished
	}
	var msg FlashStartMessage
	err := json.Unmarshal(event.Payload, &msg)
	if err != nil {
		return err
	}
	if msg.FileSize < 1 {
		return nil
	}
	if msg.FileSize > maxFileSize {
		return ErrFlashLargeFile
	}
	board, exists := detector.GetBoardSync(msg.ID)
	if !exists {
		return ErrFlashWrongID
	}
	if !detector.isFake(msg.ID) {
		updated := board.updatePortName(msg.ID)
		if updated {
			if board.IsConnected() {
				DeviceUpdatePort(msg.ID, board, c)
			} else {
				detector.DeleteBoard(msg.ID)
				DeviceUpdateDelete(msg.ID, c)
				return ErrFlashDisconnected
			}
		}
	}
	if board.IsFlashBlocked() {
		return ErrFlashBlocked
	}
	boardToFlashName := strings.ToLower(board.Type.Name)
	for _, boardName := range notSupportedBoards {
		if boardToFlashName == strings.ToLower(boardName) {
			c.sendOutgoingEventMessage(ErrNotSupported.Error(), boardName, false)
			return nil
		}
	}
	// блокировка устройства и клиента для прошивки, необходимо разблокировать после завершения прошивки
	c.StartFlashing(board, msg.FileSize)
	FlashNextBlock(c)
	// блокировка устройства и клиента для прошивки, необходимо разблокировать после завершения прошивки
	c.StartFlashing(board, msg.FileSize)
	return nil
}

// принятие блока с бинарными данными файла
func FlashBinaryBlock(event Event, c *WebSocketConnection) error {
	if !c.IsFlashing() {
		return ErrFlashNotStarted
	}

	fileCreated, err := c.FileWriter.AddBlock(event.Payload)
	if err != nil {
		return err
	}
	if fileCreated {
		// сообщение от программы avrdude
		var avrMsg string
		// сообщение об ошибке (если есть)
		var err error
		if detector.isFake(c.FlashingBoard.SerialID) {
			avrMsg, err = fakeFlash(c.FlashingBoard, c.FileWriter.GetFilePath())
		} else {
			avrMsg, err = autoFlash(c.FlashingBoard, c.FileWriter.GetFilePath())
		}
		c.avrMsg = avrMsg
		if err != nil {
			c.StopFlashing()
			return ErrAvrdude
		}
		FlashDone(c)
	} else {
		FlashNextBlock(c)
	}
	return nil
}

// отправить сообщение о том, что прошивка прошла успешна
func FlashDone(c *WebSocketConnection) {
	c.StopFlashing()
	c.sendOutgoingEventMessage(FlashDoneMsg, c.avrMsg, false)
	c.avrMsg = ""
}

// запрос на следующий блок с бинаными данными файла
func FlashNextBlock(c *WebSocketConnection) {
	c.sendOutgoingEventMessage(FlashNextBlockMsg, nil, false)
}

func newDeviceMessage(board *BoardToFlash, deviceID string) *DeviceMessage {
	boardMessage := DeviceMessage{
		deviceID,
		board.Type.Name,
		board.Type.Controller,
		board.Type.Programmer,
		board.PortName,
		board.SerialID,
	}
	return &boardMessage
}

func newUpdatedMessage(board *BoardToFlash, deviceID string) *DeviceMessage {
	boardMessage := DeviceMessage{
		deviceID,
		board.Type.Name,
		board.Type.Controller,
		board.Type.Programmer,
		board.PortName,
		board.SerialID,
	}
	return &boardMessage
}

func newDeviceUpdatePortMessage(board *BoardToFlash, deviceID string) *DeviceUpdatePortMessage {
	boardMessage := DeviceUpdatePortMessage{
		deviceID,
		board.PortName,
	}
	return &boardMessage
}

func newDeviceUpdateDeleteMessage(deviceID string) *DeviceUpdateDeleteMessage {
	boardMessage := DeviceUpdateDeleteMessage{
		deviceID,
	}
	return &boardMessage
}

func GetMaxFileSize(event Event, c *WebSocketConnection) error {
	return c.sendOutgoingEventMessage(MaxFileSizeMsg, MaxFileSizeMessage{maxFileSize}, false)
}
