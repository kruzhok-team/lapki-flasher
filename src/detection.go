package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/gousb"
	"github.com/gorilla/websocket"
	"golang.org/x/sys/windows/registry"
)

const NOT_FOUND = "NOT FOUND"

type boardView struct {
	ID         int    `json:"ID"`
	Name       string `json:"name"`
	Controller string `json:"controller"`
	Programmer string `json:"programmer"`
	PortName   string `json:"portName"`
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	boards = detect_boards()

	wsc := wsConn{}
	var err error

	// Open websocket connection.
	upgrader := websocket.Upgrader{HandshakeTimeout: time.Second * HandshakeTimeoutSecs}
	wsc.conn, err = upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error on open of websocket connection:", err)
		return
	}
	defer wsc.conn.Close()

	for i, v := range boards {
		board := boardView{
			i,
			v.Type.Name,
			v.Type.Controller,
			v.Type.Programmer,
			v.PortName,
		}
		msg, err := json.Marshal(board)
		if err != nil {
			wsc.sendStatus(400, err.Error())
			return
		}
		wsc.conn.WriteMessage(websocket.TextMessage, msg)
	}
}

func find_port_name(desc *gousb.DeviceDesc) string {
	switch OS_CUR {
	case LINUX:
		return find_port_name_linux(desc)
	case WINDOWS:
		return find_port_name_windows(desc)
	default:
		panic("Current OS isn't supported! Can't find device path!\n")
	}
}

func find_port_name_linux(desc *gousb.DeviceDesc) string {
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

func find_port_name_windows(desc *gousb.DeviceDesc) string {
	cmdPathPattern := fmt.Sprintf("USB\\VID_%s&PID_%s*", desc.Vendor.String(), desc.Product.String())
	cmdPattern := fmt.Sprintf("Get-PnpDevice -status 'ok' -InstanceId '%s' | Select-Object -Property InstanceId", cmdPathPattern)
	//fmt.Println(cmdPattern)
	cmdResult := execString("powershell", cmdPattern)
	var possiblePathes []string
	curStr := ""
	for _, v := range cmdResult {
		if v == '\n' {
			if len(curStr) > 0 && curStr[0] == 'U' {
				possiblePathes = append(possiblePathes, curStr)
			}
			curStr = ""
			continue
		}
		if v == 13 {
			continue
		}
		curStr += string(v)
	}
	for _, path := range possiblePathes {
		keyPath := fmt.Sprintf("SYSTEM\\CurrentControlSet\\Enum\\%s\\Device Parameters", path)
		key, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
		if err != nil {
			log.Fatal("Registry error:", err)
		}
		s, _, err := key.GetStringValue("PortName")
		fmt.Println("PORT NAME", s)
		if err == registry.ErrNotExist {
			fmt.Println("not exists")
			continue
		}
		if err != nil {
			log.Fatal("Get port name error:", err.Error())
		}
		return s
	}
	return NOT_FOUND
}

func detect_boards() []BoardToFlash {
	ctx := gousb.NewContext()
	defer ctx.Close()
	// list of supported vendors (should contain lower case only!)
	vid := vendor_list()
	var boards []BoardToFlash
	groups := board_list()
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
						detectedBoard.VendorID = v
						detectedBoard.Port = desc.Port
						detectedBoard.PortName = find_port_name(desc)
						detectedBoard.Type = board
						boards = append(boards, detectedBoard)
						//return true
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

func vendor_list() []string {
	// lower-case only
	vendors := []string{
		"2a03",
		"2341",
	}
	return vendors
}

func board_list() map[string][]BoardType {
	boardGroups := make(map[string][]string)
	boardGroups["2341,2a03"] = []string{
		"8037;Arduino Micro;ATmega32U4;avr109;Arduino Micro (bootloader);0037",
		"0043;Arduino Uno;ATmega328P;arduino;;",
	}
	vendorGroups := make(map[string][]BoardType)
	for vendorsStr, boardsStr := range boardGroups {
		var boards []BoardType
		for _, boardParams := range boardsStr {
			params := strings.Split(boardParams, ";")
			var board BoardType
			board.ProductID = params[0]
			board.Name = params[1]
			board.Controller = params[2]
			board.Programmer = params[3]
			board.Bootloader = params[4]
			board.BootloaderID = params[5]
			boards = append(boards, board)
		}
		vendorSep := strings.Split(vendorsStr, ",")
		for _, vendor := range vendorSep {
			vendorGroups[vendor] = boards
		}
	}
	return vendorGroups
}

// TODO: error handling
func findDevice(ctx *gousb.Context, VID, PID string, port int) *gousb.Device {
	devs, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		return VID == desc.Vendor.String() && PID == desc.Product.String() && (port == -1 || desc.Port == port)
	})
	fmt.Printf("devs: %v\n", devs)
	if err != nil {
		log.Fatalf("OpenDevices(): %v", err)
	}
	numberOfDevices := len(devs)
	if numberOfDevices == 0 {
		log.Fatalln("The device hasn't been found")
	}
	if numberOfDevices > 1 {
		for _, d := range devs {
			defer d.Close()
		}
		log.Fatalln("More than one device has been found")
	}
	return devs[0]
}
