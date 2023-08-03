// хранение данных о соединении и взаимодействие с ним
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tjgq/ticker"
)

// сообщение для отправки
type OutgoingEventMessage struct {
	// содержание сообщение
	event *Event
	// true, если нужно отправить сообщение всем клиентам
	toAll bool
}

type WebSocketConnection struct {
	wsc        *websocket.Conn
	FileWriter *FlashFileWriter
	// устройство, на которое должна установиться прошивка
	FlashingBoard *BoardToFlash
	// сообщение от avrdude
	avrMsg      string
	outgoingMsg chan OutgoingEventMessage
	// отправляет тик, когда get-list снова может быть использован
	getListCoolDown *ticker.Ticker
}

func NewWebSocket(wsc *websocket.Conn) *WebSocketConnection {
	var c WebSocketConnection
	c.wsc = wsc
	c.FlashingBoard = nil
	c.FileWriter = newFlashFileWriter()
	c.avrMsg = ""
	c.outgoingMsg = make(chan OutgoingEventMessage)
	c.getListCoolDown = ticker.New(5 * time.Second)
	return &c
}

func (c *WebSocketConnection) IsFlashing() bool {
	return c.FlashingBoard != nil
}

// блокирует устройство и запрещает клиенту прошивать другие устройства, также запускает или перезапускает FileWriter для записи данных в файл прошивки
func (c *WebSocketConnection) StartFlashing(board *BoardToFlash, fileSize int) {
	c.FlashingBoard = board
	c.FlashingBoard.SetLock(true)
	c.FileWriter.Start(fileSize)
}

// разблокирует устройство и разрешает клиенту прошивать другие устройства, удаляет файл и другие данные FileWriter
func (c *WebSocketConnection) StopFlashing() {
	if c.FlashingBoard != nil {
		c.FlashingBoard.SetLock(false)
		c.FlashingBoard = nil
		c.FileWriter.Clear()
	}
}

// отправка сообщения клиенту
// lastGetListDevice - дополнительная переменная, берётся только первое значение, остальные будут игнорироваться
func (c *WebSocketConnection) sentOutgoingEventMessage(msgType string, payload any) (err error) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Println("Marshal JSON error:", err.Error())
		return
	}
	event := Event{
		msgType,
		data,
	}
	var outgoingMsg OutgoingEventMessage
	outgoingMsg.event = &event
	switch msgType {
	case DeviceUpdateDeleteMsg, DeviceUpdatePortMsg:
		outgoingMsg.toAll = true
	default:
		outgoingMsg.toAll = false
	}
	c.outgoingMsg <- outgoingMsg
	return
}

func (c *WebSocketConnection) CoolDowm() {
	for {
		time, open := <-c.getListCoolDown.C
		fmt.Println(time)
		if !open {
			return
		}
		c.getListCoolDown.Stop()
	}
}
