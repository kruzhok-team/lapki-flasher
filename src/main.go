package main

import (
	_ "embed"
	"log"
	"net/http"
)

const NOT_FOUND = ""

// пустая структура может понадобиться в map структуре, для реализации поведения set структуры (в go нет стандартной реализации set)
type void struct{}

// находит и хранит информацию об устройствах
var detector *Detector

func printLog(v ...any) {
	if verbose {
		log.Println(v...)
	}
}

func main() {
	setupOS()
	setArgs()
	printArgsDesc()

	detector = NewDetector()
	manager := NewWebSocketManager()

	http.HandleFunc("/flasher", manager.serveWS)

	log.Fatal(http.ListenAndServe(webAddress, nil))
}
