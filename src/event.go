// обработка и отправка сообщений
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
)

var (
	ErrEventNotSupported = errors.New("this event type is not supported")
)

// Событие это сообщение, переданное через вебсокеты
type Event struct {
	// Тип сообщения
	Type string `json:"type"`
	// Данные сообщения
	Payload json.RawMessage `json:"payload"`
}

// обработчик события
type EventHandler func(event Event, c *websocket.Conn) error

type DeviceMessage struct {
	ID         string `json:"deviceID,omitempty"`
	Name       string `json:"name,omitempty"`
	Controller string `json:"controller,omitempty"`
	Programmer string `json:"programmer,omitempty"`
	PortName   string `json:"portName,omitempty"`
}

// типы сообщений (событий)
const (
	// запрос на получение списка всех устройств
	GetListMsg = "get-list"
	// описание устройства
	DeviceMsg = "device"
)

// отправка сообщения клиенту
func sentOutgoingEventMessage(msgType string, payload any, c *websocket.Conn) (err error) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Println("Marshal JSON error:", err.Error())
		return
	}
	event := Event{
		msgType,
		data,
	}
	err = c.WriteJSON(event)
	if err != nil {
		log.Println("Writing JSON error:", err.Error())
		return
	}
	return
}

// отправить клиенту список всех устройств
func GetList(event Event, c *websocket.Conn) error {
	fmt.Println("get-list")
	detector.Update()
	detector.DeleteUnused()
	IDs, boards := detector.GetBoards()
	for i := range IDs {
		err := device(IDs[i], boards[i], c)
		if err != nil {
			fmt.Println("getList() error", err.Error())
			return err
		}
	}
	return nil
}

// отправить клиенту описание устройства
func device(deviceID string, board *BoardToFlash, c *websocket.Conn) error {
	boardMessage := DeviceMessage{
		deviceID,
		board.Type.Name,
		board.Type.Controller,
		board.Type.Programmer,
		board.PortName,
	}
	err := sentOutgoingEventMessage(DeviceMsg, boardMessage, c)
	if err != nil {
		fmt.Println("device() error:", err.Error())
	}
	return err
}
