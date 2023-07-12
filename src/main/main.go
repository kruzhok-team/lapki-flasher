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
type BoardToFlash struct {
	Type     BoardType
	VendorID string
	Port     int
	PortName string
}

func (board BoardType) hasBootloader() bool {
	return board.BootloaderID != ""
}

// список доступных для прошивки устройств
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

// выполнение консольной команды с обработкой ошибок и возвращением stdout
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

/*
прошивка
TODO: Обработка ошибок Avrdude и разобраться с bootloader
*/
func flash(board BoardToFlash, file string) {
	/*
		на случай если плата прошивается через bootloader,
		не работает, так как не находит bootloader,
		требуются дополнительные разрешения на Linux и возможно на других ОС для перезагрузки
	*/
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
	// TODO: нужно добавить avrdude.exe для каждой ОС и сюда указывать путь к нему
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
		fmt.Println(tempFile.Name())
		err = os.Remove(tempFile.Name())
		if err != nil {
			fmt.Println("Can't delete temporary file: ", err.Error())
		}
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
	//setupRoutes()
}
