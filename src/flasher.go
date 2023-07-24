package main

import (
	"fmt"
)

// прошивка
func flash(board BoardToFlash, file string) {
	flash := "flash:w:" + getAbolutePath(file) + ":a"
	fmt.Println(execString("avrdude", "-p", board.Type.Controller, "-c", board.Type.Programmer, "-P", board.PortName, "-U", flash))
}
