// хранение данных о соединении и взаимодействие с ним
package main

import (
	"encoding/json"
	"errors"
	"sync"

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
	FlashingBoard *Device
	// сообщение от avrdude
	avrMsg          string
	outgoingMsg     chan OutgoingEventMessage
	getListCooldown *Cooldown
	// true, если все каналы (кроме этого) закрыты
	mu sync.Mutex
	// true каналы для передпчи данных между горутинами открыты
	closed bool
	// максимальное количество одновременно обрабатываемых запросов
	maxQueries int
	// количество запросов, которые обрабатываются в данный момент
	numQueries int
}

func NewWebSocket(wsc *websocket.Conn, getListCoolDown *Cooldown, maxQueries int) *WebSocketConnection {
	var c WebSocketConnection
	c.wsc = wsc
	c.FlashingBoard = nil
	c.FileWriter = newFlashFileWriter()
	c.avrMsg = ""
	c.outgoingMsg = make(chan OutgoingEventMessage)
	c.getListCooldown = getListCoolDown
	c.maxQueries = maxQueries
	c.numQueries = 0
	return &c
}

func (c *WebSocketConnection) addQuerry(m *WebSocketManager, event Event) {
	for c.getNumQueries() > c.getMaxQueries() {
	}
	go func() {
		// откладываем таймер, так как обновление все равно произойдёт для всех
		// FIXME: почему это действие не входит в handlers?
		if event.Type == GetListMsg {
			m.updateTicker.Stop()
			defer m.updateTicker.Start()
		}
		c.incNumQueries()
		handler, exists := m.handlers[event.Type]
		if exists {
			err := handler(event, c)
			errorHandler(err, c)
		} else {
			errorHandler(ErrEventNotSupported, c)
		}
		c.decNumQueries()
	}()
}

func (c *WebSocketConnection) getNumQueries() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.numQueries
}

func (c *WebSocketConnection) getMaxQueries() int {
	// так как значение максимума не изменяется, то блокировать переменную нету смысла
	return c.maxQueries
}

func (c *WebSocketConnection) incNumQueries() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.numQueries++
}

func (c *WebSocketConnection) decNumQueries() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.numQueries--
}

func (c *WebSocketConnection) IsFlashing() bool {
	return c.FlashingBoard != nil
}

func (c *WebSocketConnection) isClosedChan() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *WebSocketConnection) closeChan() {
	if c.isClosedChan() {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.outgoingMsg)
	c.closed = true
}

// разблокирует устройство и разрешает клиенту прошивать другие устройства, удаляет файл и другие данные FileWriter
func (c *WebSocketConnection) StopFlashing() {
	if c.FlashingBoard != nil {
		c.FlashingBoard.SetLockSync(false)
		c.FlashingBoard = nil
		c.FileWriter.Clear()
	}
}

// отправка сообщения клиенту
// toAll = true, если сообщение нужно отправить всем клиентам
// startCooldown[0] = true, если нужно запустить cooldown
func (c *WebSocketConnection) sendOutgoingEventMessage(msgType string, payload any, toAll bool, startCooldown ...bool) (err error) {
	if c.isClosedChan() {
		return errors.New("can't send message because the client is closed.")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		printLog("Marshal JSON error:", err.Error())
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
