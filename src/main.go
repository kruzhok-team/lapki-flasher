package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
)

type OS string

var OS_CUR OS

const (
	LINUX   OS = "LINUX"
	WINDOWS OS = "WINDOWS"
)

type BoardType struct {
	ProductID    string
	Name         string
	Controller   string
	Programmer   string
	Bootloader   string
	BootloaderID string
}
type BoardToFlash struct {
	Type     BoardType
	VendorID string
	Port     int
	PortName string
}

func (board BoardType) hasBootloader() bool {
	return board.BootloaderID != ""
}

// список доступных для прошивки устройств
var boards []BoardToFlash

func setupRoutes() {
	http.HandleFunc("/", showJS)
	http.HandleFunc("/list", listHandler)
	http.HandleFunc("/upload", uploadHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func main() {
	OS_CUR = WINDOWS
	vendorGroups := board_list()
	for i, v := range vendorGroups {
		fmt.Printf("i: %s v: %v\n", i, v)
	}
	fmt.Println()
	boards = detect_boards()
	for _, board := range boards {
		fmt.Printf("board: %v %t\n", board, board.Type.hasBootloader())
	}
	setupRoutes()
}
