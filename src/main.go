package main

import (
	_ "embed"
	"log"
	"net/http"
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

}
