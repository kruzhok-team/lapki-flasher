// обработка и отправка сообщений
package main

import (
	"encoding/json"
	"fmt"
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
	// клиент может начать передачу блоков
	FlashNextBlockMsg = "flash-next-block"
	// сообщение, для отметки бинарных данных загружаемого файла прошивки, прикрепляется сервером к сообщению после получения данных бинарного типа
	FlashBinaryBlockMsg = "flash-block"
	// устройство удалено из списка
	DeviceUpdateDeleteMsg = "device-update-delete"
	// устройство поменяло порт
	DeviceUpdatePortMsg = "device-update-port"
	// запрос на следующий блок бинарных данных
	flashNextBlockMsg = "flash-next-block"
	// сообщение, содержащее бинарные данные для загружаемого файла прошивки, прикрепляется сервером к сообщению после получения бинарных данных
	binaryBloMsg = "binaryMsg"
)

// отправить клиенту список всех устройств
func GetList(event Event, c *WebSocketConnection) error {
	fmt.Println("get-list")
	detector.Update()
	detector.DeleteUnused()
	IDs, boards := detector.GetBoards()
	for i := range IDs {
		err := Device(IDs[i], boards[i], c)
		if err != nil {
			fmt.Println("getList() error", err.Error())
			return err
		}
	}
	return nil
}

// отправить клиенту описание устройства
func Device(deviceID string, board *BoardToFlash, c *WebSocketConnection) error {
	fmt.Println("device")
	boardMessage := DeviceMessage{
		deviceID,
		board.Type.Name,
		board.Type.Controller,
		board.Type.Programmer,
		board.PortName,
		board.SerialID,
	}
	err := c.sentOutgoingEventMessage(DeviceMsg, boardMessage)
	if err != nil {
		fmt.Println("device() error:", err.Error())
	}
	return err
}

// сообщение о том, что порт обновлён
func DeviceUpdatePort(deviceID string, board *BoardToFlash, c *WebSocketConnection) {
	dev := DeviceUpdatePortMessage{
		deviceID,
		board.PortName,
	}
	c.sentOutgoingEventMessage(DeviceUpdatePortMsg, dev)
}

// сообщение о том, что устройство удалено
func DeviceUpdateDelete(deviceID string, c *WebSocketConnection) {
	detector.DeleteBoard(deviceID)
	dev := DeviceUpdateDeleteMessage{
		deviceID,
	}
	c.sentOutgoingEventMessage(DeviceUpdateDeleteMsg, dev)
}

// подготовка к чтению файла с прошивкой и к его загрузке на устройство
func FlashStart(event Event, c *WebSocketConnection) error {
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
	if msg.FileSize > MAX_FILE_SIZE {
		return ErrFlashLargeFile
	}
	board, exists, updated := detector.GetBoard(msg.ID)
	if !exists {
		return ErrFlashWrongID
	}
	if updated {
		if board.IsConnected() {
			DeviceUpdatePort(msg.ID, board, c)
		} else {
			DeviceUpdateDelete(msg.ID, c)
			return ErrFlashDisconnected
		}
	}
	if board.IsFlashBlocked() {
		return ErrFlashBlocked
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
		avrMsg, err := flash(c.FlashingBoard, c.FileWriter.GetFilePath())
		if err != nil {
			c.avrMsg = avrMsg
			c.StopFlashing()
			return ErrAvrdude
		}
		FlashDone(c)
	} else {
		FlashNextBlock(c)
	}
	return nil
}

// TODO: отмена прошивки
func FlashCancel(event Event, c *WebSocketConnection) error {
	if !c.IsFlashing() {
		return nil
	}
	return nil
}

// отправить сообщение о том, что прошивка прошла успешна
func FlashDone(c *WebSocketConnection) {
	c.StopFlashing()
	c.sentOutgoingEventMessage(FlashDoneMsg, nil)
}

// запрос на следующий блок с бинаными данными файла
func FlashNextBlock(c *WebSocketConnection) {
	c.sentOutgoingEventMessage(FlashNextBlockMsg, nil)
}
