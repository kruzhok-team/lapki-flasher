package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
)

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
	vendorGroups := boardList()
	for i, v := range vendorGroups {
		fmt.Printf("i: %s v: %v\n", i, v)
	}
	fmt.Println()
	boards = DetectBoards()
	for _, board := range boards {
		fmt.Printf("board: %v %t\n", board, board.Type.hasBootloader())
	}
	setupRoutes()
}
