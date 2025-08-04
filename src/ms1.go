package main

import (
	"bytes"
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
	ms1.GET_FIRMWARE:        "GET_FIRMWARE",
}

var ms1Type = map[string]string{
	"1e3204c1e573a118": "main-a3",
	"028e53ca92358dd9": "main-a4",
	"7669fba1c9175843": "mtrx-a2",
	"47af73c71f3930ce": "mtrx-a3",
	"da047a039c8acff1": "btn-a2",
	"58e2581437a30762": "btn-a2",
	"c4ef6036603a600f": "lmp-a2",
	"274b36772c9ea32a": "lmp-a4",
	"5027e18c66ac2bc3": "lmp8-a1",
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
		collectLogs(device, logger)
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

func (board *MS1) Reset() error {
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

func (board *MS1) Ping() error {
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
		return "", errors.New("не удалось обновить устройство")
	}
	return deviceMS.GetAddress(), nil
}

// Возвращает *ms1.Meta
func (board *MS1) GetMetaData() (any, error) {
	portMS, err := ms1.MkSerial(board.getFlashPort())
	if err != nil {
		return nil, err
	}
	defer portMS.Close()
	deviceMS := ms1.NewDevice(portMS)
	err = deviceMS.SetAddress(board.address)
	if err != nil {
		return nil, err
	}
	meta, err := deviceMS.GetMeta()
	if err != nil {
		printLog("meta data:", meta, " error:", err.Error())
		return &meta, err
	}
	printLog("meta data:", meta)
	return &meta, nil
}

/*
Получить тип устройства на основе RefBlHw из метаданных.

Возвращает пустую строку, если не удаётся определить тип устройства.
*/
func getMSType(RefBlHw string) string {
	postfix, ok := ms1Type[RefBlHw]
	if !ok {
		return ""
	}
	return "tjc-ms1-" + postfix
}

func metaToJSON(meta *ms1.Meta) MetaSubMessage {
	return MetaSubMessage{
		RefBlHw:       meta.RefBlHw,
		RefBlFw:       meta.RefBlFw,
		RefBlUserCode: meta.RefBlUserCode,
		RefBlChip:     meta.RefBlChip,
		RefBlProtocol: meta.RefBlProtocol,
		RefCgHw:       meta.RefCgHw,
		RefCgFw:       meta.RefCgFw,
		RefCgProtocol: meta.RefCgProtocol,
	}
}

/*
Получение адреса и затем метаданных.
Если адрес не удалось получить, то вернётся пустая строка,  nil и ошибкой.
Если метаданные не удалось получить то вернётся адрес, nil и ошибка.
*/
func (board *MS1) getAddressAndMeta() (string, *ms1.Meta, error) {
	portMS, err := ms1.MkSerial(board.getFlashPort())
	if err != nil {
		return "", nil, err
	}
	defer portMS.Close()
	deviceMS := ms1.NewDevice(portMS)
	// получение адреса
	_, err, updated := deviceMS.GetId(true, true)
	if err != nil {
		return "", nil, err
	}
	if !updated {
		return "", nil, errors.New("не удалось обновить устройство")
	}
	// получение метаданных
	meta, err := deviceMS.GetMeta()
	return deviceMS.GetAddress(), &meta, err
}

func (board *MS1) getFirmware(address string, logger chan any, RefBlChip string) ([]byte, error) {
	portMS, err := ms1.MkSerial(board.getFlashPort())
	if err != nil {
		return nil, err
	}
	defer portMS.Close()
	deviceMS := ms1.NewDevice(portMS)
	err = deviceMS.SetAddress(address)
	if err != nil {
		return nil, err
	}
	if logger != nil {
		collectLogs(deviceMS, logger)
	}
	frames := 0
	if RefBlChip == "" {
		// Присылать ли клиенту метаданные?
		meta, err := deviceMS.GetMeta()
		if err != nil {
			printLog("getFirmware: no meta:", err.Error())
		} else {
			RefBlChip = meta.RefBlChip
		}
	}
	frames = getFirmwareFrames(RefBlChip)
	if frames == 0 {
		printLog("getFirmware: no meta: can't identify RefBlChip, setting frames value as 400")
		frames = 400
	}

	var b bytes.Buffer
	err = deviceMS.GetFirmware(&b, frames)
	return b.Bytes(), err
}

func collectLogs(deviceMS *ms1.Device, logger chan any) {
	devLogger := deviceMS.ActivateLog()
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

/*
Функция возвращет максимальное количество фреймов, содержащих прошивку по параметру метаданных RefBlChip.
Если не удаётся найти подходящее значение по RefBlChip, то возвращется 0.
*/
func getFirmwareFrames(RefBlChip string) int {
	/*
		# bootloader REF_CHIP

		Указывает на контроллер, здесь то, что нужно для компиляции прошивки (вид контроллера, память, число страниц, первая страница).

		- 0xb2cc4e728f9bf8f6: STM32G030F6, 8KB RAM, 0x7-я первая страница, всего 16 страниц, страницы по 2КБ.
		- 0xb4272ba421624bbe: STM32G030K8/STM32G030C8, 8KB RAM, 0x7-я первая страница, всего 32 страницы, но доступны только 17 (до 16U включительно), страницы по 2КБ.
		- 0x387857a4b687c7f3: STM32G030C8, 8KB RAM, 0x7-я первая страница, всего 32 страницы, страницы по 2КБ.
	*/

	// В одной странице 16 фреймов.
	framesPerPage := 16
	switch RefBlChip {
	case "387857a4b687c7f3":
		return (32 - 7) * framesPerPage
	case "b4272ba421624bbe":
		return (17 - 7) * framesPerPage
	case "b2cc4e728f9bf8f6":
		return (16 - 7) * framesPerPage
	}
	return 0
}

// Возвращение всех плат, которые откликнулись на пинг
func (board *MS1) getConnectedBoards(addresses []string, client *WebSocketConnection) ([]string, error) {
	const (
		GET_BOARDS_BACKTRACK_PING       = 0
		GET_BOARDS_BACKTRACK_REPLY      = 1
		GET_BOARDS_BACKTRACK_NO_REPLY   = 2
		GET_BOARDS_BACKTRACK_WRONG_ADDR = 3
	)
	portMS, err := ms1.MkSerial(board.getFlashPort())
	if err != nil {
		return nil, err
	}
	defer portMS.Close()
	deviceMS := ms1.NewDevice(portMS)
	connectedBoards := make([]string, 0, len(addresses))
	for _, address := range addresses {
		err = deviceMS.SetAddress(address)
		if err != nil {
			MSGetConnectedBoardsBacktrack(MSGetConnectedBacktrackMessage{
				Address: address,
				Code:    GET_BOARDS_BACKTRACK_WRONG_ADDR,
			}, client)
			continue
		}
		MSGetConnectedBoardsBacktrack(MSGetConnectedBacktrackMessage{
			Address: address,
			Code:    GET_BOARDS_BACKTRACK_PING,
		}, client)
		_, err = deviceMS.Ping()
		if err != nil {
			MSGetConnectedBoardsBacktrack(MSGetConnectedBacktrackMessage{
				Address: address,
				Code:    GET_BOARDS_BACKTRACK_NO_REPLY,
			}, client)
			continue
		}
		MSGetConnectedBoardsBacktrack(MSGetConnectedBacktrackMessage{
			Address: address,
			Code:    GET_BOARDS_BACKTRACK_REPLY,
		}, client)
		connectedBoards = append(connectedBoards, address)
	}
	return connectedBoards, nil
}
