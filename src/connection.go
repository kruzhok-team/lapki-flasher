// хранение данных о соединении и взаимодействие с ним
package main

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

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
	wsc *websocket.Conn
	// устройство, на которое должна установиться прошивка
	FlashingBoard *Device
	FlashingDevId string
	// сообщение от прошивающей программы
	flasherMsg      string
	outgoingMsg     chan OutgoingEventMessage
	getListCooldown *Cooldown
	mu              sync.Mutex
	closed          bool
	// максимальное количество одновременно обрабатываемых запросов
	maxQueries int
	// количество запросов, которые обрабатываются в данный момент
	numQueries  int
	Manager     *WebSocketManager
	binDataChan chan []byte
}

func NewWebSocket(wsc *websocket.Conn, getListCooldownDuration time.Duration, m *WebSocketManager, maxQueries int) *WebSocketConnection {
	var c WebSocketConnection
	c.wsc = wsc
	c.FlashingBoard = nil
	c.FlashingDevId = ""
	c.flasherMsg = ""
	c.outgoingMsg = make(chan OutgoingEventMessage)
	c.getListCooldown = newCooldown(getListCooldownDuration, m)
	c.maxQueries = maxQueries
	c.numQueries = 0
	c.Manager = m
	c.binDataChan = make(chan []byte)
	return &c
}

func (c *WebSocketConnection) addQuerry(event Event) {
	for c.getNumQueries() > c.getMaxQueries() {
	}
	go func() {
		c.incNumQueries()
		c.handleEvent(event)
		c.decNumQueries()
	}()
}

func (c *WebSocketConnection) handleEvent(event Event) {
	manager := c.Manager
	handler, exists := manager.handlers[event.Type]
	if exists {
		err := handler(event, c)
		errorHandler(err, c)
	} else {
		errorHandler(ErrEventNotSupported, c)
	}
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

// true, если ожидается передача данных через binDataChan
func (c *WebSocketConnection) IsBinChanBusySync() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.FlashingBoard != nil
}

func (c *WebSocketConnection) SetFlashingBoard(dev *Device, ID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.FlashingBoard = dev
	c.FlashingDevId = ID
}

func (c *WebSocketConnection) GetFlashingBoardSync() *Device {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.FlashingBoard
}

func (c *WebSocketConnection) GetFlashingDevIdSync() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.FlashingDevId
}

func (c *WebSocketConnection) GetFlasherMessageSync() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.flasherMsg
}

func (c *WebSocketConnection) SetFlasherMessageSync(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flasherMsg = msg
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
func (c *WebSocketConnection) StopFlashingSync() {
	if c.FlashingBoard != nil {
		c.FlashingBoard.SetLockSync(false)
		c.FlashingBoard = nil
		c.FlashingDevId = ""
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

func (c *WebSocketConnection) sendBinaryMessage(bytes []byte, toAll bool) (err error) {
	if c.isClosedChan() {
		return errors.New("can't send message because the client is closed")
	}
	outgoingMsg := OutgoingEventMessage{
		event: &Event{
			Type:    "",
			Payload: bytes,
		},
		toAll: toAll,
	}
	c.outgoingMsg <- outgoingMsg
	return
}
