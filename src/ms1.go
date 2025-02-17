package main

import (
	"errors"
	"fmt"

	"github.com/polyus-nt/ms1-go/pkg/ms1"
)

type MS1 struct {
	portNames [4]string // должно быть ровно 4 порта
	address   string
	verify    bool  // если true, то будет проверка после прошивки
	ms1OS     MS1OS // структура с данными для поиска устройства на определённой ОС
}

var ms1backtrackStatus = map[ms1.UploadStage]string{
	ms1.PING:                "PING",
	ms1.PREPARE_FIRMWARE:    "PREPARE_FIRMWARE",
	ms1.CHANGE_MODE_TO_PROG: "CHANGE_MODE_TO_PROG",
	ms1.CHANGE_MODE_TO_RUN:  "CHANGE_MODE_TO_RUN",
	ms1.ERASE_OLD_FIRMWARE:  "ERASE_OLD_FIRMWARE",
	ms1.PUSH_FIRMWARE:       "PUSH_FIRMWARE",
	ms1.PULL_FIRMWARE:       "PULL_FIRMWARE",
	ms1.VERIFY_FIRMWARE:     "VERIFY_FIRMWARE",
}

func NewMS1(portNames [4]string, ms1OS MS1OS) *MS1 {
	ms1 := MS1{
		portNames: portNames,
		address:   "",
		verify:    false,
		ms1OS:     ms1OS,
	}
	return &ms1
}

func (board *MS1) GetSerialPort() string {
	return board.portNames[3]
}

func (board *MS1) IsConnected() bool {
	return board.portNames[0] != NOT_FOUND
}

func (board *MS1) Flash(filePath string, logger chan any) (string, error) {
	port, err := ms1.MkSerial(board.getFlashPort())
	if err != nil {
		return err.Error(), err
	}
	defer port.Close()

	device := ms1.NewDevice(port)
	if board.address != "" {
		err := device.SetAddress(board.address)
		if err != nil {
			return "Не удалось использовать адрес устройства. " + err.Error(), err
		}
	}
	if logger != nil {
		devLogger := device.ActivateLog()
		go func() {
			for log := range devLogger {
				logger <- FlashBacktrackMsMessage{
					UploadStage: ms1backtrackStatus[log.UploadStage],
					NoPacks:     log.NoPacks,
					CurPack:     log.CurPack,
					TotalPacks:  log.TotalPacks,
				}
			}
			close(logger)
		}()
	}
	packs, err := device.WriteFirmware(filePath, board.verify)
	if err != nil {
		return err.Error(), err
	}
	flashMessage := handleFlashResult(fmt.Sprint(packs), err)
	return flashMessage, err
}

func (board *MS1) getFlashPort() string {
	return board.portNames[0]
}

func (board *MS1) GetWebMessageType() string {
	return MSDeviceMsg
}

func (board *MS1) GetWebMessage(name string, deviceID string) any {
	return MSDeviceMessage{
		ID:        deviceID,
		Name:      name,
		PortNames: board.portNames,
	}
}

func (board *MS1) reset() error {
	portMS, err := ms1.MkSerial(board.getFlashPort())
	if err != nil {
		return err
	}
	defer portMS.Close()
	deviceMS := ms1.NewDevice(portMS)
	err = deviceMS.SetAddress(board.address)
	if err != nil {
		return err
	}
	deviceMS.Reset(true)
	return nil
}

func (board *MS1) ping() error {
	portMS, err := ms1.MkSerial(board.getFlashPort())
	if err != nil {
		return err
	}
	defer portMS.Close()
	deviceMS := ms1.NewDevice(portMS)
	err = deviceMS.SetAddress(board.address)
	if err != nil {
		return err
	}
	_, err = deviceMS.Ping()
	if err != nil {
		return err
	}
	return nil
}

// получить адрес для МС-ТЮК
func (board *MS1) getAddress() (string, error) {
	portMS, err := ms1.MkSerial(board.getFlashPort())
	if err != nil {
		return "", err
	}
	defer portMS.Close()
	deviceMS := ms1.NewDevice(portMS)
	_, err, updated := deviceMS.GetId(true, true)
	if err != nil {
		return "", err
	}
	if !updated {
		return "", errors.New("Не удалось обновить устройство.")
	}
	return deviceMS.GetAddress(), nil
}

func (board *MS1) getMetaData() (ms1.Meta, error) {
	portMS, err := ms1.MkSerial(board.getFlashPort())
	if err != nil {
		return ms1.Meta{}, err
	}
	defer portMS.Close()
	deviceMS := ms1.NewDevice(portMS)
	deviceMS.SetAddress(board.address)
	meta, err := deviceMS.GetMeta()
	if err != nil {
		printLog("meta data:", meta, " error:", err.Error())
		return meta, err
	}
	printLog("meta data:", meta)
	return meta, nil
}

/*
Получить тип устройства на основе RefBlHw из метаданных.

Возвращает пустую строку, если не удаётся определить тип устройства.
*/
func getMSType(RefBlHw string) string {
	switch RefBlHw {
	case "1e3204c1e573a118":
		return "tjc-ms1-main-a3"
	case "028e53ca92358dd9":
		return "tjc-ms1-main-a4"
	case "7669fba1c9175843":
		return "tjc-ms1-mtrx-a2"
	case "47af73c71f3930ce":
		return "tjc-ms1-mtrx-a3"
	case "da047a039c8acff1":
		return "tjc-ms1-btn-a2"
	case "58e2581437a30762":
		return "tjc-ms1-btn-a2"
	case "c4ef6036603a600f":
		return "tjc-ms1-lmp-a2"
	case "274b36772c9ea32a":
		return "tjc-ms1-lmp-a4"
	}
	return ""
}
