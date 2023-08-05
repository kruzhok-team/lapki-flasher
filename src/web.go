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

// максимальное количество сообщений, которые ожидают своей обработки
const MAX_WAITING_MESSAGES = 50

/*
минимальное время, через которое клиент может снова запросить список устройств;

игнорируется, если количество клиентов меньше чем 2
*/
const GET_LIST_COOLDOWN_DURATION = 5 * time.Second

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
	m.updateTicker = *ticker.New(time.Second * 5)
	m.updateTicker.Start()
	go m.updater()
	return &m
}

func (m *WebSocketManager) hasMultipleConnections() bool {
	return len(m.connections) > 1
}

// инициализация обработчиков событий
func (m *WebSocketManager) setupEventHandlers() {
	m.handlers[GetListMsg] = GetList
	m.handlers[FlashStartMsg] = FlashStart
	m.handlers[FlashBinaryBlockMsg] = FlashBinaryBlock
}

// отправляет событие в соответствующий обработчик, если для события не существует обработчика возвращает ошибку ErrEventNotSupported
func (m *WebSocketManager) routeEvent(event Event, c *WebSocketConnection) error {
	if event.Type == GetListMsg {
		m.updateTicker.Stop()
		defer m.updateTicker.Start()
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
	c := NewWebSocket(conn, newCooldown(GET_LIST_COOLDOWN_DURATION, m))
	m.addClient(c)
	defer func() {
		m.updateTicker.Stop()
		UpdateList(c, m)
		m.updateTicker.Start()
	}()
	go m.writerHandler(c)
	go m.eventHandler(c)
	go m.readerHandler(c)
}

// добавление нового клиента
func (m *WebSocketManager) addClient(c *WebSocketConnection) {
	m.connections[c] = true
}

// удаление клиента
func (m *WebSocketManager) removeClient(c *WebSocketConnection) {
	log.Println("remove client")
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
			log.Println("reader: removed")
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error reading message: %v", err)
			}
			break
		}

		var event Event
		if msgType == websocket.BinaryMessage {
			event.Type = FlashBinaryBlockMsg
			event.Payload = payload
		} else {
			err := json.Unmarshal(payload, &event)
			if err != nil {
				errorHandler(ErrUnmarshal, c)
				continue
			}
		}
		if event.Type == FlashStartMsg && c.IsFlashing() {
			errorHandler(ErrFlashNotFinished, c)
			continue
		}
		select {
		case c.readEvent <- event:
		default:
			log.Println("read channel is full!")
			if msgType == websocket.BinaryMessage {
				c.sentOutgoingEventMessage(ErrWaitingBinaryMessagesLimit.Error(), nil, false)
			} else {
				c.sentOutgoingEventMessage(ErrWaitingMessagesLimit.Error(), payload, false)
			}
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

func (m *WebSocketManager) eventHandler(c *WebSocketConnection) {
	defer func() {
		m.removeClient(c)
	}()
	for {
		event, isOpen := <-c.readEvent
		if !isOpen {
			return
		}
		if err := m.routeEvent(event, c); err != nil {
			errorHandler(err, c)
			continue
		}
	}
}

func (m *WebSocketManager) updater() {
	for {
		<-m.updateTicker.C
		log.Println("update")
		if len(m.connections) > 0 {
			UpdateList(nil, m)
		}
	}
}

func (m *WebSocketManager) sendMessageToAll(msgType string, payload any) {
	for connection := range m.connections {
		connection.sentOutgoingEventMessage(msgType, payload, false)
	}
}

func UpdateList(c *WebSocketConnection, m *WebSocketManager) {
	sendToAll := c == nil

	// замораживаем блокировки
	if sendToAll {
		for connection := range m.connections {
			connection.getListCooldown.freeze()
		}
	} else {
		c.getListCooldown.freeze()
	}
	// запускаем временные блокировки
	defer func() {
		if sendToAll {
			for connection := range m.connections {
				connection.getListCooldown.start()
			}
		} else {
			c.getListCooldown.start()
		}
	}()
	newBoards := detectBoards()
	// отправляем все устройства клиенту
	// отправляем все клиентам об изменениях в устройстве, если таковые имеются
	// отправляем всем клиентам новые устройства
	for deviceID, newBoard := range newBoards {
		oldBoard, exists := detector.GetBoard(deviceID)
		if exists {
			if oldBoard.getPort() != newBoard.PortName {
				oldBoard.setPort(newBoard.PortName)
				if sendToAll {
					m.sendMessageToAll(DeviceUpdatePortMsg, newDeviceUpdatePortMessage(newBoard, deviceID))
				} else {
					DeviceUpdatePort(deviceID, newBoard, c)
				}
			}
			if !sendToAll {
				Device(deviceID, newBoard, false, c)
			}
		} else {
			detector.AddBoard(deviceID, newBoard)
			if sendToAll {
				m.sendMessageToAll(DeviceMsg, newDeviceMessage(newBoard, deviceID))
			} else {
				Device(deviceID, newBoard, true, c)
			}
		}
	}
	detector.mu.Lock()
	for deviceID := range detector.boards {
		_, exists := newBoards[deviceID]
		if !exists {
			//deletedMsgs = append(deletedMsgs, *newDeviceUpdateDeleteMessage(deviceID))
			if sendToAll {
				m.sendMessageToAll(DeviceUpdateDeleteMsg, newDeviceUpdateDeleteMessage(deviceID))
			} else {
				DeviceUpdateDelete(deviceID, c)
			}
			delete(detector.boards, deviceID)
		}
	}
	detector.mu.Unlock()
}
