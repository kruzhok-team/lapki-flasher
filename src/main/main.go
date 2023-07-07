package main

import (
	_ "embed"
	"os"
	"strconv"

	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/gousb"
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
			log.Fatalf("Coudln't reset the device: %v\n", err)
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

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	var data UploadInfo
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	upload(data)
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

func setupRoutes() {
	http.HandleFunc("/upload/", uploadHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func main() {
	OS_CUR = LINUX
	vendorGroups := board_list()
	for i, v := range vendorGroups {
		fmt.Printf("i: %s v: %v\n", i, v)
	}
	fmt.Println()
	boards := detect_boards()
	for _, board := range boards {
		fmt.Printf("board: %v %t\n", board, board.Type.hasBootloader())
	}
	flash(boards[0], "firmwares/blinkUNO.hex")
}
