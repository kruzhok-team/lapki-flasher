// содержит функции, связанные с отправкой и обработкой сообщений
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tjgq/ticker"
)

// максмальный размер одного сообщения, передаваемого через веб-сокеты (в байтах)
const MAX_MSG_SIZE = 1024

// максимальный размер файла, загружаемого на сервер (в байтах)
const MAX_FILE_SIZE = 2 * 1024 * 1024

var (
	websocketUpgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

// список соединений с клиентами
type ConnectionList map[*WebSocketConnection]bool

// Менеджер используется для контроля и хранения соединений с клиентами
type WebSocketManager struct {
	// обработчики сообщений (событий)
	handlers map[string]EventHandler
	// список соединений
	connections ConnectionList
	// отпраляет сигнал, когда нужно обновить устройства для всех
	updateTicker ticker.Ticker
}

// Инициализация менеджера
func NewWebSocketManager() *WebSocketManager {
	var m WebSocketManager
	m.connections = make(ConnectionList)
	m.handlers = make(map[string]EventHandler)
	m.setupEventHandlers()
	m.updateTicker = *ticker.New(time.Second * 30)
	m.updateTicker.Start()
	go m.updater()
	return &m
}

// инициализация обработчиков событий
func (m *WebSocketManager) setupEventHandlers() {
	m.handlers[GetListMsg] = GetList
	m.handlers[FlashStartMsg] = FlashStart
	m.handlers[FlashBinaryBlockMsg] = FlashBinaryBlock
}

// отправляет событие в соответствующий обработчик, если для события не существует обработчика возвращает ошибку ErrEventNotSupported
func (m *WebSocketManager) routeEvent(msgType int, payload []byte, c *WebSocketConnection) error {
	var event Event
	if msgType == websocket.BinaryMessage {
		event.Type = FlashBinaryBlockMsg
		event.Payload = payload
	} else {
		err := json.Unmarshal(payload, &event)
		if err != nil {
			return ErrUnmarshal
		}
	}
	if handler, ok := m.handlers[event.Type]; ok {
		if err := handler(event, c); err != nil {
			return err
		}
		return nil
	} else {
		return ErrEventNotSupported
	}
}

// обработка нового соединения
func (m *WebSocketManager) serveWS(w http.ResponseWriter, r *http.Request) {

	log.Println("New connection")
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	c := NewWebSocket(conn)
	m.addClient(c)

	go c.CoolDowm()
	go m.readerHandler(c)
	go m.writerHandler(c)
}

// добавление нового клиента
func (m *WebSocketManager) addClient(c *WebSocketConnection) {
	m.connections[c] = true
}

// удаление клиента
func (m *WebSocketManager) removeClient(c *WebSocketConnection) {
	if _, ok := m.connections[c]; ok {
		// нужно разблокировать устройство, если прошивка ещё не завершилась
		if c.IsFlashing() {
			c.StopFlashing()
		}
		c.wsc.Close()
		delete(m.connections, c)
	}
}

// обработчик входящих сообщений
func (m *WebSocketManager) readerHandler(c *WebSocketConnection) {
	defer func() {
		m.removeClient(c)
	}()

	c.wsc.SetReadLimit(MAX_MSG_SIZE)

	for {
		msgType, payload, err := c.wsc.ReadMessage()
		if err != nil {
			// Если соединения разорвано, то получится ошибка
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error reading message: %v", err)
			}
			break
		}

		// обработка сообщений и ошибок
		if err := m.routeEvent(msgType, payload, c); err != nil {
			errorHandler(err, c)
			continue
		}
	}
}

// обработчик исходящих сообщений
func (m *WebSocketManager) writerHandler(c *WebSocketConnection) {
	defer func() {
		m.removeClient(c)
	}()
	for {
		outgoing, isOpen := <-c.outgoingMsg
		if !isOpen {
			return
		}

		// некоторые сообщения нужно отправить всем клиентам
		if outgoing.toAll {
			for conn := range m.connections {
				if conn != c {
					var curOutgoing OutgoingEventMessage
					curOutgoing.event = outgoing.event
					curOutgoing.toAll = false
					conn.outgoingMsg <- curOutgoing
				}
			}
		}
		// отправить одному клиенту
		err := c.wsc.WriteJSON(outgoing.event)
		log.Println("writer", outgoing.event.Type)
		if err != nil {
			log.Println("Writing JSON error:", err.Error())
			return
		}
	}
}

func (m *WebSocketManager) updater() {
	for {
		<-m.updateTicker.C
		newBoards := detectBoards()
		//var newDevicesMsgs []DeviceMessage
		//var portUpdatedMsgs []DeviceUpdatePortMessage
		//var deletedMsgs []DeviceUpdateDeleteMessage
		for deviceID, newBoard := range newBoards {
			oldBoard, exists := detector.GetBoard(deviceID)
			if exists {
				if oldBoard.getPort() != newBoard.PortName {
					oldBoard.setPort(newBoard.PortName)
					m.sendMessageToAll(DeviceUpdatePortMsg, newDeviceUpdatePortMessage(newBoard, deviceID))
					//portUpdatedMsgs = append(portUpdatedMsgs, *newDeviceUpdatePortMessage(newBoard, deviceID))
				}
			} else {
				detector.AddBoard(deviceID, newBoard)
				m.sendMessageToAll(DeviceMsg, newDeviceMessage(newBoard, deviceID))
				//newDevicesMsgs = append(newDevicesMsgs, *newDeviceMessage(newBoard, deviceID))
			}
		}
		detector.mu.Lock()
		for deviceID := range detector.boards {
			_, exists := newBoards[deviceID]
			if !exists {
				//deletedMsgs = append(deletedMsgs, *newDeviceUpdateDeleteMessage(deviceID))
				m.sendMessageToAll(DeviceUpdateDeleteMsg, newDeviceUpdateDeleteMessage(deviceID))
				delete(detector.boards, deviceID)
			}
		}
		detector.mu.Unlock()
	}
}

func (m *WebSocketManager) sendMessageToAll(msgType string, payload any) {
	for connection := range m.connections {
		connection.sentOutgoingEventMessage(msgType, payload, false)
	}
}
