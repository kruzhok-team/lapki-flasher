// содержит функции, связанные с отправкой и обработкой сообщений
package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

const MAX_SIZE = 512 * 1024

var (
	websocketUpgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

// список соединений с клиентами
type ConnectionList map[*websocket.Conn]bool

// Менеджер используется для контроля и хранения соединений с клиентами
type WebSocketManager struct {
	// обработчики сообщений (событий) от клиента
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
}

// отправляет событие в соответствующий обработчик, если для события не существует обработчика возвращает ошибку ErrEventNotSupported
func (m *WebSocketManager) routeEvent(event Event, c *websocket.Conn) error {
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
	m.addClient(conn)
	//
	go m.connectionHandler(conn)
}

// добавление нового клиента
func (m *WebSocketManager) addClient(connection *websocket.Conn) {
	m.connections[connection] = true
}

// удаление клиента
func (m *WebSocketManager) removeClient(connection *websocket.Conn) {
	if _, ok := m.connections[connection]; ok {
		connection.Close()
		delete(m.connections, connection)
	}
}

// обработчик соединения
func (m *WebSocketManager) connectionHandler(c *websocket.Conn) {
	defer func() {
		m.removeClient(c)
	}()
	c.SetReadLimit(MAX_SIZE)

	for {
		_, payload, err := c.ReadMessage()

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
