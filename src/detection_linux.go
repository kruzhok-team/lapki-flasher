//go:build linux
// +build linux

package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/google/gousb"
)

func findPortName(desc gousb.DeviceDesc) string {
	// <bus>-<port[.port[.port]]>:<config>.<interface> - шаблон папки в которой должен находиться путь к папке tty

	// в каком порядке идут порты? Надо проверить
	ports := strconv.Itoa(desc.Path[0])
	num_ports := len(desc.Path)
	for i := 1; i < num_ports; i++ {
		ports += ".[" + strconv.Itoa(desc.Path[i])
	}
	for i := 1; i < num_ports; i++ {
		ports += "]"
	}

	// рекурсивно проходимся по возможным config и interface до тех пор пока не найдём tty папку

	//
	dir_prefix := "/sys/bus/usb/devices"
	tty := "tty"
	for _, conf := range desc.Configs {
		for _, inter := range conf.Interfaces {
			dir := fmt.Sprintf("%s/%d-%s:%d.%d/%s", dir_prefix, desc.Bus, ports, conf.Number, inter.Number, tty)
			existance, _ := exists(dir)
			if existance {
				// использование Readdirnames вместо ReadDir может ускорить работу в 20 раз
				dirs, _ := os.ReadDir(dir)
				return fmt.Sprintf("/dev/%s", dirs[0].Name())
				//return fmt.Sprintf("%s/%s", dir, dirs[0].Name())
			}
		}

	}
	return NOT_FOUND
}

func findID(desc gousb.DeviceDesc) string {
	return ""
}
