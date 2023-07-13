package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/gousb"
	"github.com/gorilla/websocket"
)

const NOT_FOUND = "NOT FOUND"

type BoardType struct {
	ProductID    string
	Name         string
	Controller   string
	Programmer   string
	Bootloader   string
	BootloaderID string
}

type boardView struct {
	ID         int    `json:"ID"`
	Name       string `json:"name"`
	Controller string `json:"controller"`
	Programmer string `json:"programmer"`
	PortName   string `json:"portName"`
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	boards = DetectBoards()

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

	for i, v := range boards {
		board := boardView{
			i,
			v.Type.Name,
			v.Type.Controller,
			v.Type.Programmer,
			v.PortName,
		}
		msg, err := json.Marshal(board)
		if err != nil {
			wsc.sendStatus(400, err.Error())
			return
		}
		wsc.conn.WriteMessage(websocket.TextMessage, msg)
	}
}

func DetectBoards() []BoardToFlash {
	ctx := gousb.NewContext()
	defer ctx.Close()
	// list of supported vendors (should contain lower case only!)
	vid := vendorList()
	var boards []BoardToFlash
	groups := boardList()
	_, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		// this function is called for every device present.
		for _, v := range vid {
			if strings.ToLower(desc.Vendor.String()) == strings.ToLower(v) {
				//fmt.Println(v, desc.Product)
				cur_group := groups[v]
				var detectedBoard BoardToFlash
				//fmt.Println(len(cur_group), v)
				for _, board := range cur_group {
					if strings.ToLower(board.ProductID) == strings.ToLower(desc.Product.String()) {
						detectedBoard.VendorID = v
						detectedBoard.Port = desc.Port
						detectedBoard.PortName = findPortName(desc)
						detectedBoard.Type = board
						boards = append(boards, detectedBoard)
						//return true
						break
					}
				}
			}
		}
		return false
	})
	if err != nil {
		log.Fatalf("OpenDevices(): %v", err)
	}
	//fmt.Println(d)
	return boards
}

func vendorList() []string {
	// lower-case only
	vendors := []string{
		"2a03",
		"2341",
	}
	return vendors
}

func boardList() map[string][]BoardType {
	boardGroups := make(map[string][]string)
	boardGroups["2341,2a03"] = []string{
		"8037;Arduino Micro;ATmega32U4;avr109;Arduino Micro (bootloader);0037",
		"0043;Arduino Uno;ATmega328P;arduino;;",
	}
	vendorGroups := make(map[string][]BoardType)
	for vendorsStr, boardsStr := range boardGroups {
		var boards []BoardType
		for _, boardParams := range boardsStr {
			params := strings.Split(boardParams, ";")
			var board BoardType
			board.ProductID = params[0]
			board.Name = params[1]
			board.Controller = params[2]
			board.Programmer = params[3]
			board.Bootloader = params[4]
			board.BootloaderID = params[5]
			boards = append(boards, board)
		}
		vendorSep := strings.Split(vendorsStr, ",")
		for _, vendor := range vendorSep {
			vendorGroups[vendor] = boards
		}
	}
	return vendorGroups
}

// TODO: error handling
func findDevice(ctx *gousb.Context, VID, PID string, port int) *gousb.Device {
	devs, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		return VID == desc.Vendor.String() && PID == desc.Product.String() && (port == -1 || desc.Port == port)
	})
	fmt.Printf("devs: %v\n", devs)
	if err != nil {
		log.Fatalf("OpenDevices(): %v", err)
	}
	numberOfDevices := len(devs)
	if numberOfDevices == 0 {
		log.Fatalln("The device hasn't been found")
	}
	if numberOfDevices > 1 {
		for _, d := range devs {
			defer d.Close()
		}
		log.Fatalln("More than one device has been found")
	}
	return devs[0]
}
