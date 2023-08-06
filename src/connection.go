// хранение данных о соединении и взаимодействие с ним
package main

import (
	"encoding/json"
	"log"
	"os/exec"

	"github.com/gorilla/websocket"
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
	// канал для прочитанных сообщений от клиента
	readEvent       chan Event
	getListCooldown *Cooldown
	flashProcces    *exec.Cmd
}

func NewWebSocket(wsc *websocket.Conn, getListCoolDown *Cooldown) *WebSocketConnection {
	var c WebSocketConnection
	c.wsc = wsc
	c.FlashingBoard = nil
	c.FileWriter = newFlashFileWriter()
	c.avrMsg = ""
	c.outgoingMsg = make(chan OutgoingEventMessage)
	c.readEvent = make(chan Event, MAX_WAITING_MESSAGES)
	c.getListCooldown = getListCoolDown
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
		if c.flashProcces != nil {
			c.flashProcces.Process.Kill()
		}
		c.flashProcces = nil
	}
}

// отправка сообщения клиенту
// toAll = true, если сообщение нужно отправить всем клиентам
// startCooldown[0] = true, если нужно запустить cooldown
func (c *WebSocketConnection) sentOutgoingEventMessage(msgType string, payload any, toAll bool, startCooldown ...bool) (err error) {
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
	outgoingMsg.toAll = toAll
	c.outgoingMsg <- outgoingMsg
	return
}
