package main

import (
	_ "embed"
	"encoding/json"
	"io/ioutil"
	"os"
	"strconv"

	//"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/gousb"
	"github.com/gorilla/websocket"
	//"github.com/xela07ax/XelaGoDoc/encodingStdout"
)

type OS string

var OS_CUR OS

const (
	LINUX   OS = "LINUX"
	WINDOWS OS = "WINDOWS"
)

type BoardType struct {
	ProductID    string
	Name         string
	Controller   string
	Programmer   string
	Bootloader   string
	BootloaderID string
}

func (board BoardType) hasBootloader() bool {
	return board.BootloaderID != ""
}

type BoardToFlash struct {
	Type     BoardType
	VendorID string
	Port     int
	PortName string
}

type UploadInfo struct {
	Controller string `json:"Controller"`
	Programmer string `json:"Programmer"`
	PortReset  string `json:"PortReset"`
	PortUpload string `json:"PortUpload"`
	FilePath   string `json:"FilePath"`
}

var boards []BoardToFlash

// exists returns whether the given file or directory exists
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func execString(name string, arg ...string) string {
	fmt.Println(name, arg)
	cmd := exec.Command(name, arg...)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + string(stdout))
		return ""
	}
	return string(stdout)
}

func getAbolutePath(path string) string {
	abspath, err := filepath.Abs(path)
	if err != nil {
		fmt.Println(err.Error())
		return ""
	}
	return abspath
}

func reset(port string) {
	switch OS_CUR {
	case WINDOWS:
		execString(getAbolutePath("src/OS/Windows/reset.bat"), port)
	default:
		panic("Current OS isn't supported! Can't reset the device!\n")
	}
}

func transfer(controller, programmer, portUpload, filePath string) {
	flash := "flash:w:" + getAbolutePath(filePath) + ":a"
	fmt.Println(execString(getAbolutePath("avrdude/avrdude.exe"), "-p", controller, "-c", programmer, "-P", portUpload, "-U", flash))
}

func upload(data UploadInfo) {
	reset(data.PortReset)
	time.Sleep(time.Second)
	transfer(data.Controller, data.Programmer, data.PortUpload, data.FilePath)
}

// todo: error handling
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

func flash(board BoardToFlash, file string) {
	if board.Type.hasBootloader() {
		ctx := gousb.NewContext()
		dev := findDevice(ctx, board.VendorID, board.Type.ProductID, board.Port)
		err := dev.Reset()
		if err != nil {
			fmt.Printf("Coudln't reset the device: %v\n", err)
			return
		}
		time.Sleep(time.Second)
		ctxNew := gousb.NewContext()
		bootloader := findDevice(ctxNew, board.VendorID, board.Type.BootloaderID, -1)
		fmt.Println(bootloader)
		dev.Close()
		ctx.Close()
		bootloader.Close()
		ctxNew.Close()
	}
	flash := "flash:w:" + getAbolutePath(file) + ":a"
	//fmt.Println(execString(getAbolutePath("avrdude/avrdude.exe"), "-p", controller, "-c", programmer, "-P", portUpload, "-U", flash))
	fmt.Println(execString("avrdude", "-p", board.Type.Controller, "-c", board.Type.Programmer, "-P", board.PortName, "-U", flash))
}

// https://gist.github.com/tsilvers/5f827fb11aee027e22c6b3102ebcc497

const HandshakeTimeoutSecs = 10

type UploadHeader struct {
	Filename string
	Size     int
	BoardID  int
}

type UploadStatus struct {
	Code   int    `json:"code,omitempty"`
	Status string `json:"status,omitempty"`
	Pct    *int   `json:"pct,omitempty"` // File processing AFTER upload is done.
	pct    int
}

type wsConn struct {
	conn *websocket.Conn
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
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
	// Get upload file name and length.
	header := new(UploadHeader)
	mt, message, err := wsc.conn.ReadMessage()
	if err != nil {
		fmt.Println("Error receiving websocket message:", err)
		return
	}
	if mt != websocket.TextMessage {
		wsc.sendStatus(400, "Invalid message received, expecting file name and length")
		return
	}
	err = json.Unmarshal(message, header)
	if err != nil {
		wsc.sendStatus(400, "Error receiving file name, length and board ID: "+err.Error())
		return
	}
	if len(header.Filename) == 0 {
		wsc.sendStatus(400, "Filename cannot be empty")
		return
	}
	if header.Size == 0 {
		wsc.sendStatus(400, "Upload file is empty")
		return
	}
	if header.BoardID < 0 || header.BoardID > len(boards) {
		wsc.sendStatus(400, "Wrong id")
		return
	}
	// Create temp file to save file.
	var tempFile *os.File
	if tempFile, err = ioutil.TempFile("tmp", "upload-*.hex"); err != nil {
		wsc.sendStatus(400, "Could not create temp file: "+err.Error())
		return
	}
	defer func() {
		tempFile.Close()
		// *** IN PRODUCTION FILE SHOULD BE REMOVED AFTER PROCESSING ***
		// _ = os.Remove(tempFile.Name())
	}()
	// Read file blocks until all bytes are received.
	bytesRead := 0
	for {
		mt, message, err := wsc.conn.ReadMessage()
		if err != nil {
			wsc.sendStatus(400, "Error receiving file block: "+err.Error())
			return
		}
		if mt != websocket.BinaryMessage {
			if mt == websocket.TextMessage {
				if string(message) == "CANCEL" {
					wsc.sendStatus(400, "Upload canceled")
					return
				}
			}
			wsc.sendStatus(400, "Invalid file block received")
			return
		}

		tempFile.Write(message)

		bytesRead += len(message)
		if bytesRead == header.Size {
			tempFile.Close()
			break
		}

		wsc.requestNextBlock()

	}

	flash(boards[header.BoardID], tempFile.Name())
	//flash(boards[0], tempFile.Name())
	wsc.sendStatus(200, "Upload successful: "+fmt.Sprintf("%s (%d bytes)", tempFile.Name(), bytesRead))
}

func (wsc wsConn) requestNextBlock() {
	wsc.conn.WriteMessage(websocket.TextMessage, []byte("NEXT"))
}

func (wsc wsConn) sendStatus(code int, status string) {
	if msg, err := json.Marshal(UploadStatus{Code: code, Status: status}); err == nil {
		wsc.conn.WriteMessage(websocket.TextMessage, msg)
	}
}

func (wsc wsConn) sendPct(pct int) {
	stat := UploadStatus{pct: pct}
	stat.Pct = &stat.pct
	if msg, err := json.Marshal(stat); err == nil {
		wsc.conn.WriteMessage(websocket.TextMessage, msg)
	}
}

func showJS(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "webpage.html")
}

func find_port_name(desc *gousb.DeviceDesc) string {
	switch OS_CUR {
	case LINUX:
		return find_port_name_linux(desc)
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
	return ""
}

//go:embed device_list.txt
var boards_list_str string

//go:embed vendors.txt
var vendors_list_str string

func detect_boards() []BoardToFlash {
	ctx := gousb.NewContext()
	defer ctx.Close()
	// list of supported vendors (should contain lower case only!)
	vid := strings.Split(strings.ToLower(vendors_list_str), "\n")
	var boards []BoardToFlash
	groups := board_list()
	_, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		// this function is called for every device present.
		for _, v := range vid {
			if strings.ToLower(desc.Vendor.String()) == v {
				//fmt.Println(v, desc.Product)
				cur_group := groups[v]
				var detectedBoard BoardToFlash
				//fmt.Println(len(cur_group), v)
				for _, board := range cur_group {
					if board.ProductID == desc.Product.String() {
						detectedBoard.VendorID = v
						detectedBoard.Port = desc.Port
						detectedBoard.PortName = find_port_name(desc)
						detectedBoard.Type = board
						boards = append(boards, detectedBoard)
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
	return boards
}

func board_list() map[string][]BoardType {
	vendorGroups := make(map[string][]BoardType)
	splitGroups := strings.Split(boards_list_str, ".\n")
	//fmt.Println(splitGroups, len(splitGroups))
	n := len(splitGroups) - 1
	for i, v := range splitGroups {
		if i == n {
			break
		}
		//fmt.Println(v, len(v))
		strs := strings.Split(v, "\n")
		var cur_vendors []string
		boards := make([]BoardType, len(strs)-1)
		for j, s := range strs {
			//fmt.Println(j, s)
			if j == 0 {
				cur_vendors = strings.Split(s, ",")
			} else {
				params := strings.Split(s, ";")
				index := j - 1
				boards[index].ProductID = params[0]
				boards[index].Name = params[1]
				boards[index].Controller = params[2]
				boards[index].Programmer = params[3]
				boards[index].Bootloader = params[4]
				boards[index].BootloaderID = params[5]
			}
		}
		for _, vendor := range cur_vendors {
			vendorGroups[vendor] = boards
		}
	}
	return vendorGroups
}

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

func setupRoutes() {
	http.HandleFunc("/", showJS)
	http.HandleFunc("/list", listHandler)
	http.HandleFunc("/upload", uploadHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func main() {
	OS_CUR = LINUX
	vendorGroups := board_list()
	for i, v := range vendorGroups {
		fmt.Printf("i: %s v: %v\n", i, v)
	}
	fmt.Println()
	boards = detect_boards()
	for _, board := range boards {
		fmt.Printf("board: %v %t\n", board, board.Type.hasBootloader())
	}
	setupRoutes()
}
