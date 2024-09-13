//go:build linux

package main

import (
	"fmt"
	stdlog "log"
	"os"
	"os/exec"
	"strconv"
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
func detectBoards(boardTemplates []BoardTemplate) map[string]*BoardFlashAndSerial {
	ctx := gousb.NewContext()
	defer ctx.Close()

	boards := make(map[string]*BoardFlashAndSerial)

	_, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		// this function is called for every device present.
		for _, boardTemplate := range boardTemplates {
			for _, vid := range boardTemplate.VendorIDs {
				if strings.ToLower(desc.Vendor.String()) == strings.ToLower(vid) {
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
								boardTemplate.IsMSDevice,
							}
							detectedBoard := NewBoardToFlash(boardType, findPortName(desc))
							if detectedBoard.getPort() == NOT_FOUND {
								printLog("can't find port")
								continue
							}
							properties, err := findProperty(detectedBoard.getPort(), USEC_INITIALIZED, ID_SERIAL)
							if err != nil {
								printLog("can't find ID", err.Error())
								continue
							}
							detectedBoard.SerialID = properties[1]
							var id string
							if detectedBoard.SerialID != NOT_FOUND && !detectedBoard.Type.IsMSDevice {
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

/*
true - если порт изменился или не найден, иначе false

назначает порту значение NOT_FOUND, если не удалось найти порт

TODO: переделать интерфейс функции для всех платформ, сделать, чтобы функция возвращала error

TODO: обновление нескольких портов
*/
func (board *BoardFlashAndSerial) updatePortName(ID string) bool {
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
	return answers, nil
}

// перезагрузка порта
func rebootPort(portName string) (err error) {
	// stty 115200 -F /dev/ttyUSB0 raw -echo
	cmd := exec.Command("stty", "1200", "-F", portName, "raw", "-echo")
	_, err = cmd.CombinedOutput()
	if err != nil {
		printLog(cmd.Args, err)
	}
	return err
}
