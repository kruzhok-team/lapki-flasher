//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/google/gousb"
)

func findPortName(desc *gousb.DeviceDesc) string {
	// <bus>-<port[.port[.port]]>:<config>.<interface> - шаблон папки в которой должен находиться путь к папке tty

	// в каком порядке идут порты? Надо проверить
	ports := strconv.Itoa(desc.Path[0])
	num_ports := len(desc.Path)
	for i := 1; i < num_ports; i++ {
		ports += "." + strconv.Itoa(desc.Path[i])
	}

	// рекурсивно проходимся по возможным config и interface до тех пор пока не найдём tty папку

	//
	dir_prefix := "/sys/bus/usb/devices"
	tty := "tty"
	for _, conf := range desc.Configs {
		for _, inter := range conf.Interfaces {
			dir := fmt.Sprintf("%s/%d-%s:%d.%d/%s", dir_prefix, desc.Bus, ports, conf.Number, inter.Number, tty)
			printLog("DIR", dir)
			existance, _ := exists(dir)
			if existance {
				// использование Readdirnames вместо ReadDir может ускорить работу в 20 раз
				dirs, _ := os.ReadDir(dir)
				return fmt.Sprintf("%s/%s", DEV, dirs[0].Name())
				//return fmt.Sprintf("%s/%s", dir, dirs[0].Name())
			}
			printLog(dir, "doesn't exists")
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
