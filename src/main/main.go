package main

import (
	_ "embed"

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

type Board struct {
	ProductID    string
	VendorID     string
	Name         string
	Controller   string
	Programmer   string
	Bootloader   string
	BootloaderID string
	Port         int
}

type UploadInfo struct {
	Controller string `json:"Controller"`
	Programmer string `json:"Programmer"`
	PortReset  string `json:"PortReset"`
	PortUpload string `json:"PortUpload"`
	FilePath   string `json:"FilePath"`
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
	execString(getAbolutePath("src/OS/Windows/reset.bat"), port)
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

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	var data UploadInfo
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	upload(data)
}

func detect_boards() []Board {
	ctx := gousb.NewContext()
	defer ctx.Close()
	// list of supported vendors (should contain lower case only!)
	vid := []string{"2341", "2a03"}
	var boards []Board
	_, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		//fmt.Println(desc.Product.String(), desc.Vendor.String())
		// this function is called for every device present.
		for _, v := range vid {
			if strings.ToLower(desc.Vendor.String()) == v {
				var board Board
				board.ProductID = desc.Product.String()
				board.Port = desc.Port
				board.VendorID = desc.Vendor.String()
				boards = append(boards, board)
				break
			}
		}
		return false
	})
	if err != nil {
		log.Fatalf("OpenDevices(): %v", err)
	}
	return boards
}

//go:embed device_list.txt
var str string

func board_list() {
	/*content, err := os.ReadFile("device_list.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(content))*/
	fmt.Println(str)
}

func setupRoutes() {
	http.HandleFunc("/upload/", uploadHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func main() {
	/*board := detect_boards()
	for i := range board {
		fmt.Println(board[i].ProductID, board[i].Port)
	}*/
	board_list()
}
