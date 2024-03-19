//go:build linux || darwin

package main

import (
	stdlog "log"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/google/gousb"
)

const DEV = "/dev"
const ID_SERIAL = "ID_SERIAL_SHORT"
const USEC_INITIALIZED = "USEC_INITIALIZED"

// настройка для выбранной ОС
func setupOS() {
	stdlog.SetOutput(new(LogrusWriter))

}

// удаление сообщения "interrupted [code -10]" из консоли

type LogrusWriter int

const interruptedError = "interrupted [code -10]"

func (LogrusWriter) Write(data []byte) (int, error) {
	logmessage := string(data)
	if strings.Contains(logmessage, interruptedError) {
		log.Tracef("gousb_logs:%s", logmessage)
		return len(data), nil
	}
	log.Infof("gousb_logs:%s", logmessage)
	return len(data), nil
}

// находит все подключённые платы
func detectBoards(boardTemplates []BoardTemplate) map[string]*BoardToFlash {
	// start := time.Now()
	// defer fmt.Println("detection time: ", time.Now().Sub(start))
	ctx := gousb.NewContext()
	defer ctx.Close()

	boards := make(map[string]*BoardToFlash)

	_, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		// this function is called for every device present.
		for _, boardTemplate := range boardTemplates {
			for _, vid := range boardTemplate.VendorIDs {
				if strings.ToLower(desc.Vendor.String()) == strings.ToLower(vid) {
					//fmt.Println(v, desc.Product)
					//fmt.Println(len(cur_group), v)
					for _, pid := range boardTemplate.ProductIDs {
						if strings.ToLower(pid) == strings.ToLower(desc.Product.String()) {
							boardType := BoardType{
								boardTemplate.ID,
								pid,
								vid,
								boardTemplate.Name,
								boardTemplate.Controller,
								boardTemplate.Programmer,
								boardTemplate.BootloaderID,
							}
							detectedBoard := NewBoardToFlash(boardType, findPortName(desc))
							if detectedBoard.PortName == NOT_FOUND {
								printLog("can't find port")
								continue
							}
							properties, err := findProperty(detectedBoard.PortName, USEC_INITIALIZED, ID_SERIAL)
							if err != nil {
								printLog("can't find ID", err.Error())
								continue
							}
							detectedBoard.SerialID = properties[1]
							var id string
							if detectedBoard.SerialID != NOT_FOUND {
								id = detectedBoard.SerialID
							} else {
								id = properties[0]
							}
							boards[id] = detectedBoard
						}
					}
				}
			}
		}
		return false
	})
	if err != nil {
		log.Printf("OpenDevices(): %v\n", err)
		return nil
	}
	return boards
}

// true - если порт изменился или не найден, иначе false
// назначает порту значение NOT_FOUND, если не удалось найти порт
func (board *BoardToFlash) updatePortName(ID string) bool {
	// start := time.Now()
	// defer fmt.Println("update port time", time.Now().Sub(start))
	var properties []string
	var err error
	if board.SerialID == NOT_FOUND {
		properties, err = findProperty(board.getPortSync(), USEC_INITIALIZED)
	} else {
		properties, err = findProperty(board.getPortSync(), ID_SERIAL)
	}
	printLog(board.Type.ProductID, board.Type.ProductID)
	if err == nil && properties[0] == ID {
		return false
	}
	newPortName := NOT_FOUND
	if board.SerialID == NOT_FOUND {
		return true
	}
	ctx := gousb.NewContext()
	defer ctx.Close()
	ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if desc.Product.String() == board.Type.ProductID && desc.Vendor.String() == board.Type.VendorID {
			portName := findPortName(desc)
			properties, _ = findProperty(portName, ID_SERIAL)
			printLog("prop", properties)
			if properties[0] == board.SerialID {
				newPortName = portName
				return false
			}
		}
		return false
	})
	board.setPortSync(newPortName)
	return true
}

// перезагрузка порта
func rebootPort(portName string) (err error) {
	// stty 115200 -F /dev/ttyUSB0 raw -echo
	cmd := exec.Command("stty", "1200", "-F", portName, "raw", "-echo")
	_, err = cmd.CombinedOutput()
	printLog(cmd.Args, err)
	return err
}
