// обработка и отправка сообщений
package main

import (
	"encoding/json"
	"fmt"
)

// обработчик события
type EventHandler func(event Event, c *WebSocketConnection) error

// Событие это сообщение, переданное через вебсокеты
type Event struct {
	// Тип сообщения
	Type string `json:"type"`
	// Данные сообщения
	Payload json.RawMessage `json:"payload"`
}

type DeviceMessage struct {
	ID         string `json:"deviceID"`
	Name       string `json:"name,omitempty"`
	Controller string `json:"controller,omitempty"`
	Programmer string `json:"programmer,omitempty"`
	PortName   string `json:"portName,omitempty"`
}

type FlashStartMessage struct {
	ID       string `json:"deviceID"`
	FileSize int    `json:"fileSize"`
}

type FlashBlockMessage struct {
	BlockID int    `json:"blockID"`
	Data    []byte `json:"data"`
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
	FlashGoMsg = "flash-go"
	// часть файла с прошивкой
	FLashBlockMsg = "flash-block"
	// устройство удалено из списка
	DeviceUpdateDeleteMsg = "device-update-delete"
	// устройство поменяло порт
	DeviceUpdatePortMsg = "device-update-port"
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
	fmt.Println("FLASH-START")
	if c.IsFlashing() {
		return ErrFlashNotFinished
	}
	var msg FlashStartMessage
	err := json.Unmarshal(event.Payload, &msg)
	fmt.Println(msg.ID)
	for i := range detector.boards {
		fmt.Println(i)
	}
	if err != nil {
		return err
	}
	if msg.FileSize < 1 {
		return nil
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
	FlashGo(c)
	// блокировка устройства и клиента для прошивки, необходимо разблокировать после завершения прошивки
	c.StartFlashing(board, msg.FileSize)
	return nil
}

// принятие блока с данными файла
func FlashBlock(event Event, c *WebSocketConnection) error {
	if !c.IsFlashing() {
		return ErrFlashNotStarted
	}
	var msg *FlashBlockMessage
	fmt.Println(event.Payload)

	err := json.Unmarshal(event.Payload, msg)
	if err != nil {
		return err
	}
	fileCreated, err := c.FileWriter.AddBlock(msg)
	if err != nil {
		return err
	}
	if fileCreated {
		err := flash(c.FlashingBoard, c.FileWriter.filePath)
		if err != nil {
			return err
		}
		FlashDone(c)
	}
	return nil
}

// отмена прошивки
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

// клиент может начать передачу блоков
func FlashGo(c *WebSocketConnection) {
	c.sentOutgoingEventMessage(FlashGoMsg, nil)
}
