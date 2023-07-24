//go:build linux
// +build linux

package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/google/gousb"
)

func detectBoards() map[string]BoardToFlash {
	ctx := gousb.NewContext()
	defer ctx.Close()
	// list of supported vendors (should contain lower case only!)
	vid := vendorList()
	boards := make(map[string]BoardToFlash)
	groups := boardList()

	_, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		// this function is called for every device present.
		for _, v := range vid {
			if strings.ToLower(desc.Vendor.String()) == strings.ToLower(v) {
				//fmt.Println(v, desc.Product)
				cur_group := groups[v]
				var detectedBoard BoardToFlash
				//fmt.Println(len(cur_group), v)
				for _, board := range cur_group {
					if strings.ToLower(board.ProductID) == strings.ToLower(desc.Product.String()) {
						detectedBoard.PortName = findPortName(desc)
						if detectedBoard.PortName == NOT_FOUND {
							continue
						}
						detectedBoard.Type = board
						detectedBoard.IsConnected = true
						boardID := findID(desc)
						boards[boardID] = detectedBoard
						break
					}
				}
			}
		}
		return false
	})
	if err != nil {
		log.Fatalf("OpenDevices(): %v", err)
	}
	//fmt.Println(d)
	return boards
}

func findPortName(desc gousb.DeviceDesc) string {
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
			fmt.Println("DIR", dir)
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

func (board *BoardToFlash) updatePortName(ID string) bool {
	return false
}
