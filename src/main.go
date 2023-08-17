package main

import (
	_ "embed"
	"log"
	"net/http"
)

//go:embed index.html
var staticPage []byte

const webPort = ":8080"

var detector *Detector

func showJS(w http.ResponseWriter, r *http.Request) {
	w.Write(staticPage)
}

func main() {
	detector = NewDetector()

	manager := NewWebSocketManager()

	http.HandleFunc("/", showJS)
	http.HandleFunc("/flasher", manager.serveWS)

	log.Fatal(http.ListenAndServe(webPort, nil))
}
