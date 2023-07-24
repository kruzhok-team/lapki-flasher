package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"time"
)

const webPort = ":8080"

func setupRoutes() {
	//http.HandleFunc("/", showJS)
	//http.HandleFunc("/upload", uploadHandler)
	log.Fatal(http.ListenAndServe(webPort, nil))
}

func main() {
	/*vendorGroups := boardList()
	for i, v := range vendorGroups {
		fmt.Printf("i: %s v: %v\n", i, v)
	}
	fmt.Println()
	boards = DetectBoards()
	for _, board := range boards {
		fmt.Printf("board: %v %t\n", board, board.Type.hasBootloader())
	}*/
	//setupRoutes()
	d := New()
	d.Update()
	for i, v := range d.boards {
		fmt.Println(i, v, d.IsNew(i), v.IsConnected())
	}
	d.Update()
	for i, v := range d.boards {
		fmt.Println(i, v, d.IsNew(i), v.IsConnected())
	}
	fmt.Println("PAUSE")
	time.Sleep(8 * time.Second)
	d.Update()
	for i, v := range d.boards {
		fmt.Println(i, v, d.IsNew(i), v.IsConnected())
	}
	d.DeleteUnused()
	for i, v := range d.boards {
		fmt.Println(i, v, d.IsNew(i), v.IsConnected())
	}
	fmt.Println("PAUSE")
	time.Sleep(8 * time.Second)
	d.Update()
	for i, v := range d.boards {
		fmt.Println(i, v, d.IsNew(i), v.IsConnected())
	}
	d.Update()
	for i, v := range d.boards {
		fmt.Println(i, v, d.IsNew(i), v.IsConnected())
	}
}
