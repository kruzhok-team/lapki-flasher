package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
)

const HandshakeTimeoutSecs = 10

type UploadStatus struct {
	Code   int    `json:"code,omitempty"`
	Status string `json:"status,omitempty"`
}

type wsConn struct {
	conn *websocket.Conn
}

func (wsc wsConn) requestNextBlock() {
	wsc.conn.WriteMessage(websocket.TextMessage, []byte("NEXT"))
}

func (wsc wsConn) sendStatus(code int, status string) {
	if msg, err := json.Marshal(UploadStatus{Code: code, Status: status}); err == nil {
		wsc.conn.WriteMessage(websocket.TextMessage, msg)
	}
}

func showJS(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "webpage.html")
}
