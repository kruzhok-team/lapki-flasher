package main

import (
	_ "embed"
	"log"
	"net/http"
	"os"
)

//go:embed index.html
var staticPage []byte

var webPort string

const DEFAULT_PORT = ":8080"

var detector *Detector

func showJS(w http.ResponseWriter, r *http.Request) {
	w.Write(staticPage)
}

func main() {
	detector = NewDetector()

	manager := NewWebSocketManager()

	if len(os.Args) > 1 {
		webPort = os.Args[1]
	} else {
		webPort = DEFAULT_PORT
	}
	http.HandleFunc("/", showJS)
	http.HandleFunc("/flasher", manager.serveWS)

	log.Fatal(http.ListenAndServe(webPort, nil))
}
