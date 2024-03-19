//go:build darwin

package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/google/gousb"
)

func findPortName(desc *gousb.DeviceDesc) string {
	pid := desc.Product.String()
	vid := desc.Vendor.String()

	cmd := exec.Command("system_profiler", "SPUSBDataType")
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		printLog(string(stdout), err.Error())
		return NOT_FOUND
	}
	lines := strings.Split(string(stdout), "\n")

	var ttyPort, deviceName string

	for _, line := range lines {
		if strings.Contains(line, fmt.Sprintf("Product ID: %d", pid)) && strings.Contains(line, fmt.Sprintf("Vendor ID: %d", vid)) {
			// Extract the device name
			deviceName = strings.TrimSpace(strings.Split(line, "Location ID:")[0])
			break
		}
	}

	if deviceName != "" {
		cmd = exec.Command("ioreg", "-p", "IOUSB", "-l")
		stdout, err = cmd.Output()
		if err != nil {
			printLog(string(stdout), err.Error())
			return NOT_FOUND
		}

		lines = strings.Split(string(stdout), "\n")

		for _, line := range lines {
			if strings.Contains(line, deviceName) {
				ttyPort = strings.TrimSpace(strings.Split(line, "IODialinDevice")[1])
				return ttyPort
			}
		}
	}
	return NOT_FOUND
}

// возвращает значение указанных параметров устройства, подключённого к порту portName,
// можно использовать для того, чтобы получить серийный номер устройства (если есть) или для получения времени, когда устройство было подключено (используется как ID)
//
//	см. "udevadm info --query=propery" для большей информации об параметрах
func findProperty(portName string, properties ...string) ([]string, error) {
	numProperties := len(properties)
	if numProperties == 0 {
		return nil, nil
	}
	cmd := exec.Command("udevadm", "info", "--query=property", "--name="+portName)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		printLog(string(stdout), err.Error())
		return nil, err
	}
	lines := strings.Split(string(stdout), "\n")
	var answers = make([]string, numProperties)
	for _, line := range lines {
		lineSize := len(line)
		for i, property := range properties {
			if answers[i] != "" {
				continue
			}
			propertySize := len(property)
			if propertySize > lineSize {
				continue
			}
			if line[:propertySize] == property {
				answers[i] = line[propertySize+1:]
			}
		}
	}
	//fmt.Println(id)
	return answers, nil
}
