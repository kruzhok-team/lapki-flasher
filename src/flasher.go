package main

import (
	"fmt"
)

// прошивка
func flash(board *BoardToFlash, filePath string) error {
	flash := "flash:w:" + getAbolutePath(filePath) + ":a"
	fmt.Println(execString("avrdude", "-p", board.Type.Controller, "-c", board.Type.Programmer, "-P", board.PortName, "-U", flash))
	return nil
}
