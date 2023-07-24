package main

// https://gist.github.com/tsilvers/5f827fb11aee027e22c6b3102ebcc497
import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type messageType string

type UploadStatus struct {
	Code   int    `json:"code,omitempty"`
	Status string `json:"status,omitempty"`
}

type deviceMessage struct {
	MessageType messageType `json:"messageType"`
	ID          string      `json:"ID,omitempty"`
	Name        string      `json:"name,omitempty"`
	Controller  string      `json:"controller,omitempty"`
	Programmer  string      `json:"programmer,omitempty"`
	PortName    string      `json:"portName,omitempty"`
	IsConnected bool        `json:"isConnected,omitempty"`
	//IsAvailable bool   `json:"isAvailable"`
}

type wsConn struct {
	conn     *websocket.Conn
	w        http.ResponseWriter
	r        *http.Request
	flashing bool
	detector Detector
}

const (
	getList           messageType = "get-list"
	flashStart        messageType = "flash-start"
	flashBlock        messageType = "flash-block"
	flashCancel       messageType = "flash-cancel"
	device            messageType = "device"
	flashWrondID      messageType = "flash-wrong-id"
	flashDisconnected messageType = "flash-disconnected"
	flashAvrdudeError messageType = "flash-avrdude-error"
	flashDone         messageType = "flash-done"
)

const HandshakeTimeoutSecs = 10

//go:embed webpage.html
var staticPage []byte

func (wsc wsConn) requestNextBlock() {
	wsc.conn.WriteMessage(websocket.TextMessage, []byte("NEXT"))
}

func (wsc wsConn) sendStatus(code int, status string) {
	if msg, err := json.Marshal(UploadStatus{Code: code, Status: status}); err == nil {
		wsc.conn.WriteMessage(websocket.TextMessage, msg)
	}
}

func showJS(w http.ResponseWriter, r *http.Request) {
	w.Write(staticPage)
}

func flasherHandler(w http.ResponseWriter, r *http.Request) {
	wsc := wsConn{}
	wsc.w = w
	wsc.r = r
	wsc.flashing = false
	wsc.detector = New()
	var err error
	// Open websocket connection.
	upgrader := websocket.Upgrader{HandshakeTimeout: time.Second * HandshakeTimeoutSecs}
	wsc.conn, err = upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error on open of websocket connection:", err)
		return
	}
	defer wsc.conn.Close()
}

// Обработка сообщений от клиента

func (wsc *wsConn) getList() {
	wsc.detector.Update()
	IDs, boards := wsc.detector.GetBoards()
	for i := range IDs {
		err := wsc.device(IDs[i], boards[i])
		if err != nil {
			fmt.Println("getList() error")
		}
	}
}

func (wsc *wsConn) flashStart() {

}

func (wsc *wsConn) flashBlock() {

}

func (wsc *wsConn) flashCancel() {

}

// Отправка сообщений клиенту

func (wsc *wsConn) device(deviceID string, board *BoardToFlash) error {
	boardMessage := deviceMessage{
		device,
		deviceID,
		board.Type.Name,
		board.Type.Controller,
		board.Type.Programmer,
		board.PortName,
		board.IsConnected(),
	}
	err := wsc.conn.WriteJSON(boardMessage)
	if err != nil {
		fmt.Println("device() error", err.Error())
	}
	return err
}

func (wsc *wsConn) flashWrongID() {

}

func (wsc *wsConn) flashDisconnected() {

}

func (wsc *wsConn) flashAvrdudeError() {

}

func (wsc *wsConn) flashDone() {

}
