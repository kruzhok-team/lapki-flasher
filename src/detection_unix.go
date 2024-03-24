//go:build linux || darwin

package main

import "os/exec"

// перезагрузка порта
func rebootPort(portName string) (err error) {
	// stty 115200 -F /dev/ttyUSB0 raw -echo
	cmd := exec.Command("stty", "1200", "-F", portName, "raw", "-echo")
	_, err = cmd.CombinedOutput()
	printLog(cmd.Args, err)
	return err
}
