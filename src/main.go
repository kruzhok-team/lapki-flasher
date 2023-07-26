package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"time"
)

//go:embed index.html
var staticPage []byte

const webPort = ":8080"

var detector *Detector

func setupRoutes() {
	//http.HandleFunc("/", showJS)
	//http.HandleFunc("/upload", uploadHandler)
	log.Fatal(http.ListenAndServe(webPort, nil))
}

func showJS(w http.ResponseWriter, r *http.Request) {
	w.Write(staticPage)
}

func test() {
	/*start := time.Now()
	detector.Update()
	end := time.Now()
	fmt.Println(end.Sub(start))*/
	start := time.Now()
	detectBoards()
	end := time.Now()
	fmt.Println(end.Sub(start))
}

func main() {
	detector = NewDetector()
	//test()
	manager := NewWebSocketManager()

	http.HandleFunc("/", showJS)
	http.HandleFunc("/flasher", manager.serveWS)

	log.Fatal(http.ListenAndServe(webPort, nil))
}
