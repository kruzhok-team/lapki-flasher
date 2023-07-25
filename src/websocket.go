package main

// https://gist.github.com/tsilvers/5f827fb11aee027e22c6b3102ebcc497
/*import (
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

type FlashMessage struct {
	MessageType messageType `json:"messageType"`
	FileSize    int         `json:"fileSize,omitempty"`
	DeviceID    string      `json:"deviceID,omitempty"`
	BlockID     string      `json:"blockID,omitempty"`
	Data        []byte      `json:"data,omitempty"`
}

type FlashError struct {
	MessageType messageType `json:"messageType"`
	AvrMsg      string      `json:"avrMsg,omitempty"`
}

type FileFlash struct {
	size       int
	fileBlocks []FileBlock
}

func (file FileFlash) flashing() bool {
	return file.size > 0
}

type FileBlock struct {
	blockID int
	data    []byte
}

type wsConn struct {
	conn     *websocket.Conn
	w        http.ResponseWriter
	r        *http.Request
	detector *Detector
	file     FileFlash
}

const (
	//getList           messageType = "get-list"
	flashStart        messageType = "flash-start"
	flashBlock        messageType = "flash-block"
	flashCancel       messageType = "flash-cancel"
	deviceDelete      messageType = "device-delete"
	flashWrongID      messageType = "flash-wrong-id"
	flashDisconnected messageType = "flash-disconnected"
	flashAvrdudeError messageType = "flash-avrdude-error"
	flashNotFinish    messageType = "flash-not-finish"
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
	wsc.detector = &detector
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

// отправка клиенту списка всех устройств
func (wsc *wsConn) getList() {
	wsc.detector.Update()
	wsc.detector.DeleteUnused()
	IDs, boards := wsc.detector.GetBoards()
	for i := range IDs {
		err := wsc.device(IDs[i], boards[i])
		if err != nil {
			fmt.Println("getList() error")
		}
	}
}

// начало процедуры прошивки
func (wsc *wsConn) flashStart() {
	if wsc.file.flashing() {
		wsc.flashNotFinish()
		return
	}

}

func (wsc *wsConn) flashBlock() {

}

func (wsc *wsConn) flashCancel() {

}

func (wsc *wsConn) deviceDelete(deviceID string) {
	var boardMessage DeviceMessage
	boardMessage.MessageType = deviceDelete
	boardMessage.ID = deviceID
	err := wsc.conn.WriteJSON(boardMessage)
	if err != nil {
		fmt.Println("deviceDelete() error", err.Error())
	}
}

func (wsc *wsConn) flashWrongID() {

}

func (wsc *wsConn) flashDisconnected() {

}

func (wsc *wsConn) flashAvrdudeError() {

}

func (wsc *wsConn) flashDone() {

}

// клиент пытается начать новую прошиввку, хотя старая ещё не завершилась
func (wsc *wsConn) flashNotFinish() {
	msg := FlashError{
		flashNotFinish,
		"",
	}
	wsc.conn.WriteJSON(msg)
}
*/
