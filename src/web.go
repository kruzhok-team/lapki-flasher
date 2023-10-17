// содержит функции, связанные с отправкой и обработкой сообщений
package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/tjgq/ticker"
)

var (
	websocketUpgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
)

// Менеджер используется для контроля и хранения соединений с клиентами
type WebSocketManager struct {
	// обработчики сообщений (событий)
	handlers map[string]EventHandler
	// список соединений
	connections *syncLenMap
	// отпраляет сигнал, когда нужно обновить устройства для всех
	updateTicker ticker.Ticker
}

// Инициализация менеджера
func NewWebSocketManager() *WebSocketManager {
	var m WebSocketManager
	m.connections = initSyncLenMap()
	m.handlers = make(map[string]EventHandler)
	m.setupEventHandlers()
	m.updateTicker = *ticker.New(updateListTime)
	m.updateTicker.Start()
	go m.updater()
	return &m
}

func (m *WebSocketManager) hasMultipleConnections() bool {
	return (m.connections.Len() > 1)
}

// инициализация обработчиков событий
func (m *WebSocketManager) setupEventHandlers() {
	m.handlers[GetListMsg] = GetList
	m.handlers[FlashStartMsg] = FlashStart
	m.handlers[FlashBinaryBlockMsg] = FlashBinaryBlock
	m.handlers[GetMaxFileSizeMsg] = GetMaxFileSize
}

// обработка нового соединения
func (m *WebSocketManager) serveWS(w http.ResponseWriter, r *http.Request) {
	printLog("New connection")
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	c := NewWebSocket(conn, newCooldown(getListCooldownDuration, m), maxThreadsPerClient)
	m.addClient(c)
	defer func() {
		m.updateTicker.Stop()
		UpdateList(c, m)
		m.updateTicker.Start()
	}()
	go m.writerHandler(c)
	go m.readerHandler(c)
}

// добавление нового клиента
func (m *WebSocketManager) addClient(c *WebSocketConnection) {
	m.connections.Add(c, true)
}

// удаление клиента
// если устройство не прошилось, то оно продолжит прошиваться и затем разблокируется
func (m *WebSocketManager) removeClient(c *WebSocketConnection) {
	printLog("remove client")
	if m.connections.Remove(c) {
		c.wsc.Close()
		c.closeChan()
	}
}

// обработчик входящих сообщений
func (m *WebSocketManager) readerHandler(c *WebSocketConnection) {
	defer func() {
		m.removeClient(c)
	}()

	c.wsc.SetReadLimit(int64(maxMsgSize))
	for {
		if c.isClosedChan() {
			return
		}
		msgType, payload, err := c.wsc.ReadMessage()
		if err != nil {
			printLog("reader: removed")
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
		c.addQuerry(m, event)
	}
}

// обработчик исходящих сообщений
func (m *WebSocketManager) writerHandler(c *WebSocketConnection) {
	defer func() {
		printLog("writer: removed")
		m.removeClient(c)
	}()
	for {
		outgoing, isOpen := <-c.outgoingMsg
		if !isOpen {
			return
		}

		// некоторые сообщения нужно отправить всем клиентам
		if outgoing.toAll {
			m.connections.Range(func(conn *WebSocketConnection, value bool) {
				if conn != c {
					var curOutgoing OutgoingEventMessage
					curOutgoing.event = outgoing.event
					curOutgoing.toAll = false
					conn.outgoingMsg <- curOutgoing
				}
			})
		}
		// отправить одному клиенту
		err := c.wsc.WriteJSON(outgoing.event)
		printLog("writer", outgoing.event.Type)
		if err != nil {
			log.Println("Writing JSON error:", err.Error())
			return
		}
	}
}

func (m *WebSocketManager) updater() {
	for {
		<-m.updateTicker.C
		if alwaysUpdate || m.connections.Len() > 0 {
			printLog("update")
			UpdateList(nil, m)
		}
	}
}

func (m *WebSocketManager) sendMessageToAll(msgType string, payload any) {
	m.connections.Range(func(connection *WebSocketConnection, value bool) {
		connection.sendOutgoingEventMessage(msgType, payload, false)
	})
}

func UpdateList(c *WebSocketConnection, m *WebSocketManager) {
	sendToAll := c == nil

	// замораживаем блокировки
	if sendToAll {
		m.connections.Range(func(connection *WebSocketConnection, value bool) {
			connection.getListCooldown.freeze()
		})
	} else {
		c.getListCooldown.freeze()
	}
	// запускаем временные блокировки
	defer func() {
		if sendToAll {
			m.connections.Range(func(connection *WebSocketConnection, value bool) {
				connection.getListCooldown.start()
			})
		} else {
			c.getListCooldown.start()
		}
	}()
	newBoards := detectBoards()
	// добавление фальшивых плат к действительно обнаруженным
	if fakeBoardsNum > 0 {
		if newBoards == nil {
			newBoards = make(map[string]*BoardToFlash)
		}
		for ID, board := range detector.fakeBoards {
			newBoards[ID] = board
		}
	}
	// отправляем все устройства клиенту
	// отправляем все клиентам об изменениях в устройстве, если таковые имеются
	// отправляем всем остальным клиентам только новые устройства
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
