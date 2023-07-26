// содержит функции, связанные с отправкой и обработкой сообщений
package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

const MAX_FILE_SIZE = 512 * 1024

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
	handlers    map[string]EventHandler
	connections ConnectionList
}

// Инициализация менеджера
func NewWebSocketManager() *WebSocketManager {
	m := &WebSocketManager{
		connections: make(ConnectionList),
		handlers:    make(map[string]EventHandler),
	}
	m.setupEventHandlers()
	return m
}

// инициализация обработчиков событий
func (m *WebSocketManager) setupEventHandlers() {
	m.handlers[GetListMsg] = GetList
	m.handlers[FlashStartMsg] = FlashStart
	m.handlers[FLashBlockMsg] = FlashBlock
}

// отправляет событие в соответствующий обработчик, если для события не существует обработчика возвращает ошибку ErrEventNotSupported
func (m *WebSocketManager) routeEvent(event Event, c *WebSocketConnection) error {
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
	//
	go m.connectionHandler(c)
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

// обработчик соединения
func (m *WebSocketManager) connectionHandler(c *WebSocketConnection) {
	defer func() {
		m.removeClient(c)
	}()
	c.wsc.SetReadLimit(MAX_FILE_SIZE)

	for {
		_, payload, err := c.wsc.ReadMessage()

		if err != nil {
			// Если соединения разорвано, то получится ошибка
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error reading message: %v", err)
			}
			break
		}
		// получение сообщения от клиента
		var request Event
		if err := json.Unmarshal(payload, &request); err != nil {
			log.Printf("error marshalling message: %v", err)
			// TODO: здесь нужно отправить сообщению клиенту о том, что не удалось прочитать сообщение
			continue
		}
		// обработка сообщений
		if err := m.routeEvent(request, c); err != nil {
			log.Println("Error handeling Message: ", err)
			continue
		}
	}
}
