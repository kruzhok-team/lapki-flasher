package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/google/gousb"
	"github.com/gorilla/websocket"
)

type BoardToFlash struct {
	Type     BoardType
	VendorID string
	Port     int
	PortName string
}

// https://gist.github.com/tsilvers/5f827fb11aee027e22c6b3102ebcc497

type UploadHeader struct {
	Filename string
	Size     int
	BoardID  int
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	wsc := wsConn{}
	var err error

	// Open websocket connection.
	upgrader := websocket.Upgrader{HandshakeTimeout: time.Second * HandshakeTimeoutSecs}
	wsc.conn, err = upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error on open of websocket connection:", err)
		return
	}
	defer wsc.conn.Close()
	// Get upload file name and length.
	header := new(UploadHeader)
	mt, message, err := wsc.conn.ReadMessage()
	if err != nil {
		fmt.Println("Error receiving websocket message:", err)
		return
	}
	if mt != websocket.TextMessage {
		wsc.sendStatus(400, "Invalid message received, expecting file name and length")
		return
	}
	err = json.Unmarshal(message, header)
	if err != nil {
		wsc.sendStatus(400, "Error receiving file name, length and board ID: "+err.Error())
		return
	}
	if len(header.Filename) == 0 {
		wsc.sendStatus(400, "Filename cannot be empty")
		return
	}
	if header.Size == 0 {
		wsc.sendStatus(400, "Upload file is empty")
		return
	}
	if header.BoardID < 0 || header.BoardID > len(boards) {
		wsc.sendStatus(400, "Wrong id")
		return
	}

	// Create temp file to save file.
	var tempFile *os.File
	if tempFile, err = ioutil.TempFile("", "upload-*.hex"); err != nil {
		wsc.sendStatus(400, "Could not create temp file: "+err.Error())
		return
	}
	defer func() {
		tempFile.Close()
		fmt.Println(tempFile.Name())
		err = os.Remove(tempFile.Name())
		if err != nil {
			fmt.Println("Can't delete temporary file: ", err.Error())
		}
	}()
	// Read file blocks until all bytes are received.
	bytesRead := 0
	for {
		mt, message, err := wsc.conn.ReadMessage()
		if err != nil {
			wsc.sendStatus(400, "Error receiving file block: "+err.Error())
			return
		}
		if mt != websocket.BinaryMessage {
			if mt == websocket.TextMessage {
				if string(message) == "CANCEL" {
					wsc.sendStatus(400, "Upload canceled")
					return
				}
			}
			wsc.sendStatus(400, "Invalid file block received")
			return
		}

		tempFile.Write(message)

		bytesRead += len(message)
		if bytesRead == header.Size {
			tempFile.Close()
			break
		}

		wsc.requestNextBlock()

	}

	flash(boards[header.BoardID], tempFile.Name())
	//flash(boards[0], tempFile.Name())
	wsc.sendStatus(200, "Upload successful: "+fmt.Sprintf("%s (%d bytes)", tempFile.Name(), bytesRead))
}

/*
прошивка
TODO: Обработка ошибок Avrdude и разобраться с bootloader
*/
func flash(board BoardToFlash, file string) {
	/*
		на случай если плата прошивается через bootloader,
		не работает, так как не находит bootloader,
		требуются дополнительные разрешения на Linux и возможно на других ОС для перезагрузки
	*/
	if board.Type.hasBootloader() {
		ctx := gousb.NewContext()
		dev := findDevice(ctx, board.VendorID, board.Type.ProductID, board.Port)
		err := dev.Reset()
		if err != nil {
			fmt.Printf("Coudln't reset the device: %v\n", err)
			return
		}
		time.Sleep(time.Second)
		ctxNew := gousb.NewContext()
		bootloader := findDevice(ctxNew, board.VendorID, board.Type.BootloaderID, -1)
		fmt.Println(bootloader)
		dev.Close()
		ctx.Close()
		bootloader.Close()
		ctxNew.Close()
	}
	flash := "flash:w:" + getAbolutePath(file) + ":a"
	//fmt.Println(execString(getAbolutePath("avrdude/avrdude.exe"), "-p", controller, "-c", programmer, "-P", portUpload, "-U", flash))
	// TODO: нужно добавить avrdude.exe для каждой ОС и сюда указывать путь к нему
	fmt.Println(execString(getAbolutePath("avrdude/avrdude.exe"), "-p", board.Type.Controller, "-c", board.Type.Programmer, "-P", board.PortName, "-U", flash))
}

func reset(port string) {
	switch OS_CUR {
	case WINDOWS:
		execString(getAbolutePath("src/OS/Windows/reset.bat"), port)
	default:
		panic("Current OS isn't supported! Can't reset the device!\n")
	}
}
