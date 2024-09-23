//go:build linux

package main

import (
	"fmt"
	"io/fs"
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
							detectedBoard := NewBoardToFlashPorts(boardType, findPortName(desc))
							if detectedBoard.getPort() == NOT_FOUND {
								printLog("can't find port")
								continue
							}
							properties, err := findProperty(detectedBoard.getPort(), USEC_INITIALIZED, ID_SERIAL)
							if err != nil {
								printLog("can't find ID", err.Error())
								continue
							}
							serialID := properties[1]
							var id string
							// на данный момент у всех МС-ТЮК одинаковый serialID, поэтому мы его игнорируем
							if serialID != NOT_FOUND && !detectedBoard.Type.IsMSDevice {
								id = detectedBoard.SerialID
								detectedBoard.SerialID = serialID
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
*/
func (board *BoardFlashAndSerial) updatePortName(ID string) bool {
	// TODO: сделать проверку для МС-ТЮК
	if board.isMSDevice() {
		return false
	}
	var properties []string
	var err error
	if board.SerialID == NOT_FOUND {
		properties, err = findProperty(board.getPortSync(), USEC_INITIALIZED)
	} else {
		properties, err = findProperty(board.getPortSync(), ID_SERIAL)
	}
	if err != nil {
		board.setPort(NOT_FOUND)
		return true
	}
	if err == nil && properties[0] == ID {
		return false
	}
	if board.SerialID == NOT_FOUND {
		return true
	}
	ctx := gousb.NewContext()
	defer ctx.Close()
	var portNames []string
	ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if desc.Product.String() == board.Type.ProductID && desc.Vendor.String() == board.Type.VendorID {
			portNames = findPortName(desc)
			foundSerialID := false
			for _, portName := range portNames {
				properties, _ = findProperty(portName, ID_SERIAL)
				printLog("prop", properties)
				if properties[0] == board.SerialID {
					foundSerialID = true
				}
			}
			if foundSerialID {
				return false
			}
		}
		return false
	})
	board.setPortsSync(portNames)
	return true
}

/*
Возвращает порты устройства, возвращает nil, если ничего не удалось найти.

Для arduino-подобных устройств должен вернуть один порт, для МС-ТЮК - четыре порта.
*/
func findPortName(desc *gousb.DeviceDesc) []string {
	// <bus>-<port[.port[.port]]>:<config>.<interface> - шаблон папки в которой должен находиться путь к папке tty

	ports := strconv.Itoa(desc.Path[0])
	num_ports := len(desc.Path)
	for i := 1; i < num_ports; i++ {
		ports += "." + strconv.Itoa(desc.Path[i])
	}

	// рекурсивно проходимся по возможным config и interface до тех пор пока не найдём tty папку
	dir_prefix := "/sys/bus/usb/devices"
	tty := "tty"
	var portNames []string
	for _, conf := range desc.Configs {
		for _, inter := range conf.Interfaces {
			root := fmt.Sprintf("%s/%d-%s:%d.%d", dir_prefix, desc.Bus, ports, conf.Number, inter.Number)
			fileSystem := os.DirFS(root)
			var devicePath string
			fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					log.Fatal(err)
				}
				fmt.Println("WALK_DIR", path, d.Name())
				if d.Name() == tty {
					// использование Readdirnames вместо ReadDir может ускорить работу в 20 раз
					dirs, err := os.ReadDir(root + "/" + path)
					if err != nil {
						return err
					}
					if len(dirs) < 1 {
						return nil
					}
					devicePath = fmt.Sprintf("%s/%s", DEV, dirs[0].Name())
					return fs.SkipAll
				}
				return nil
			})
			portNames = append(portNames, devicePath)
		}
	}
	return portNames
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
