// обработка и отправка сообщений
package main

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/polyus-nt/ms1-go/pkg/ms1"
)

// обработчик события
type EventHandler func(event Event, c *WebSocketConnection) error

// Общий вид для всех сообщений, как от клиента так и от сервера
// (исключение: бинарные данные от клиента, но все равно они приводятся сервером к этой структуре)
type Event struct {
	// Тип сообщения (flash-start, get-list и т.д.)
	Type string `json:"type"`
	// Параметры сообщения, не все сообщения обязаны иметь параметры
	Payload json.RawMessage `json:"payload"`
}

type DeviceMessage struct {
	ID         string   `json:"deviceID"`
	Name       string   `json:"name,omitempty"`
	Controller string   `json:"controller,omitempty"`
	Programmer string   `json:"programmer,omitempty"`
	PortName   string   `json:"portName,omitempty"`  // если содержится только один порт
	PortNames  []string `json:"portNames,omitempty"` // если содержится несколько портов (TODO: не самая рациональное решение, стоит придумать что-то другое)
	SerialID   string   `json:"serialID,omitempty"`
}

// тип данных для flash-start и ms-bin-start
type FlashStartMessage struct {
	ID       string `json:"deviceID"`
	FileSize int    `json:"fileSize"`
	Address  string `json:"address"`
}

type FlashBlockMessage struct {
	BlockID int    `json:"blockID"`
	Data    []byte `json:"data"`
}

type DeviceUpdateDeleteMessage struct {
	ID string `json:"deviceID"`
}

type DeviceUpdatePortMessage struct {
	ID       string `json:"deviceID"`
	PortName string `json:"portName"`
}

type MaxFileSizeMessage struct {
	Size int `json:"size"`
}

// тип данных для serial-connect и serial-change-baud
type SerialBaudMessage struct {
	ID   string `json:"deviceID"`
	Baud int    `json:"baud"`
}

type SerialDisconnectMessage struct {
	ID string `json:"deviceID"`
}

type DeviceCommentCodeMessage struct {
	ID      string `json:"deviceID"`
	Code    int    `json:"code"`
	Comment string `json:"comment"`
}

// тип данных для serial-device-read и serial-send
type SerialMessage struct {
	ID  string `json:"deviceID"`
	Msg string `json:"msg"`
}

type MSPingMessage struct {
	ID      string `json:"deviceID"`
	Address string `json:"address"`
}

type MSPingResultMessage struct {
	ID   string `json:"deviceID"`
	Code int    `json:"code"`
}

type MSGetAddressMessage struct {
	ID string `json:"deviceID"`
}

type DeviceIdMessage struct {
	ID string `json:"deviceID"`
}

// типы сообщений (событий)
const (
	// запрос на получение списка всех устройств
	GetListMsg = "get-list"
	// описание устройства
	DeviceMsg = "device"
	// запрос на прошивку устройства
	FlashStartMsg = "flash-start"
	// прошивка прошла успешна
	FlashDoneMsg = "flash-done"
	// запрос на следующий блок бинарных данных
	FlashNextBlockMsg = "flash-next-block"
	// сообщение, для отметки бинарных данных загружаемого файла прошивки, прикрепляется сервером к сообщению после получения данных бинарного типа
	FlashBinaryBlockMsg = "flash-block"
	// устройство удалено из списка
	DeviceUpdateDeleteMsg = "device-update-delete"
	// устройство поменяло порт
	DeviceUpdatePortMsg = "device-update-port"
	GetMaxFileSizeMsg   = "get-max-file-size"
	MaxFileSizeMsg      = "max-file-size"
	// устройства не найдены
	EmptyListMsg = "empty-list"
	// запрос на запуск монитора порта
	SerialConnectMsg = "serial-connect"
	// статус соединения с устройством (монитора порта)
	SerialConnectionStatusMsg = "serial-connection-status"
	// закрыть монитор порта
	SerialDisconnectMsg = "serial-disconnect"
	// запрос на отправку сообщения на устройство
	SerialSendMsg = "serial-send"
	// статус отправленного сообщения на устройство (удалось ли его отправить или нет)
	SerialSentStatusMsg = "serial-sent-status"
	// сообщение от устройства
	SerialDeviceReadMsg = "serial-device-read"
	// сменить бод
	SerialChangeBaudMsg = "serial-change-baud"
	// запрос на прошивку МС-ТЮК по адресу
	MSBinStartMsg = "ms-bin-start"
	// пинг МС-ТЮК по адресу
	MSPingMsg = "ms-ping"
	// результат выполнения команды пинг
	MSPingResultMsg = "ms-ping-result"
	// получение адреса из МС-ТЮК
	MSGetAddressMsg = "ms-get-address"
	// передача адреса из МС-ТЮК клиенту
	MSAddressMsg = "ms-address"
)

// отправить клиенту список всех устройств
func GetList(event Event, c *WebSocketConnection) error {
	printLog("get-list")
	if c.getListCooldown.isBlocked() {
		return ErrGetListCoolDown
	}
	UpdateList(c, nil)
	if detector.boardsNum() == 0 {
		c.sendOutgoingEventMessage(EmptyListMsg, nil, false)
	}
	return nil
}

// отправить клиенту описание устройства
// lastGetListDevice - дополнительная переменная, берётся только первое значение, остальные будут игнорироваться
func Device(deviceID string, board *BoardFlashAndSerial, toAll bool, c *WebSocketConnection) error {
	//printLog("device")
	boardMessage := deviceMessageMakeSync(deviceID, board)
	err := c.sendOutgoingEventMessage(DeviceMsg, boardMessage, toAll)
	if err != nil {
		printLog("device() error:", err.Error())
	}
	return err
}

// сообщение о том, что порт обновлён
func DeviceUpdatePort(deviceID string, board *BoardFlashAndSerial, c *WebSocketConnection) {
	c.sendOutgoingEventMessage(DeviceUpdatePortMsg, newDeviceUpdatePortMessage(board, deviceID), true)
}

// сообщение о том, что устройство удалено
func DeviceUpdateDelete(deviceID string, c *WebSocketConnection) {
	c.sendOutgoingEventMessage(DeviceUpdateDeleteMsg, newDeviceUpdateDeleteMessage(deviceID), true)
}

// подготовка к чтению файла с прошивкой и к его загрузке на устройство
func FlashStart(event Event, c *WebSocketConnection) error {
	log.Println("Flash-start")
	if c.IsFlashing() {
		return ErrFlashNotFinished
	}
	var msg FlashStartMessage
	err := json.Unmarshal(event.Payload, &msg)
	if err != nil {
		return err
	}
	if msg.FileSize < 1 {
		return nil
	}
	if msg.FileSize > maxFileSize {
		return ErrFlashLargeFile
	}
	board, exists := detector.GetBoardSync(msg.ID)
	if !exists {
		return ErrFlashWrongID
	}
	if !detector.isFake(msg.ID) {
		updated := board.updatePortName(msg.ID)
		if updated {
			if board.IsConnectedSync() {
				DeviceUpdatePort(msg.ID, board, c)
			} else {
				detector.DeleteBoard(msg.ID)
				DeviceUpdateDelete(msg.ID, c)
				return ErrFlashDisconnected
			}
		}
	}
	// плата блокируется!!!
	// не нужно использовать sync функции внутри блока
	board.mu.Lock()
	defer board.mu.Unlock()
	if board.IsFlashBlocked() {
		return ErrFlashBlocked
	}
	if board.isSerialMonitorOpen() {
		return ErrFlashOpenSerialMonitor
	}
	boardToFlashName := strings.ToLower(board.Type.Name)
	for _, boardName := range notSupportedBoards {
		if boardToFlashName == strings.ToLower(boardName) {
			c.sendOutgoingEventMessage(ErrNotSupported.Error(), boardName, false)
			return nil
		}
	}
	if board.isMSDevice() && event.Type != MSBinStartMsg {
		// TODO
	}
	if !board.isMSDevice() && event.Type != FlashStartMsg {
		// TODO
	}
	// блокировка устройства и клиента для прошивки, необходимо разблокировать после завершения прошивки
	c.FlashingBoard = board
	c.FlashingBoard.SetLock(true)
	if board.refToBoot != nil {
		board.refToBoot.SetLock(true)
	}
	var ext string
	if event.Type == FlashStartMsg {
		ext = "hex"
	} else {
		if msg.Address != "" {
			board.setAddressMS(msg.Address)
		} else {
			// TODO: сообщить о том, что МС-ТЮК должен знать адрес для прошивки
		}
		ext = "bin"
	}
	c.FileWriter.Start(msg.FileSize, ext)

	FlashNextBlock(c)
	return nil
}

// принятие блока с бинарными данными файла
func FlashBinaryBlock(event Event, c *WebSocketConnection) error {
	if !c.IsFlashing() {
		return ErrFlashNotStarted
	}

	fileCreated, err := c.FileWriter.AddBlock(event.Payload)
	if err != nil {
		return err
	}
	if fileCreated {
		// сообщение от программы avrdude
		var avrMsg string
		// сообщение об ошибке (если есть)
		var err error
		if detector.isFake(c.FlashingBoard.SerialID) {
			avrMsg, err = fakeFlash(c.FlashingBoard, c.FileWriter.GetFilePath())
		} else {
			avrMsg, err = autoFlash(c.FlashingBoard, c.FileWriter.GetFilePath())
		}
		c.avrMsg = avrMsg
		if err != nil {
			c.StopFlashing()
			return ErrAvrdude
		}
		FlashDone(c)
	} else {
		FlashNextBlock(c)
	}
	return nil
}

// отправить сообщение о том, что прошивка прошла успешна
func FlashDone(c *WebSocketConnection) {
	c.StopFlashing()
	c.sendOutgoingEventMessage(FlashDoneMsg, c.avrMsg, false)
	c.avrMsg = ""
}

// запрос на следующий блок с бинаными данными файла
func FlashNextBlock(c *WebSocketConnection) {
	c.sendOutgoingEventMessage(FlashNextBlockMsg, nil, false)
}

func newDeviceUpdatePortMessage(board *BoardFlashAndSerial, deviceID string) *DeviceUpdatePortMessage {
	boardMessage := DeviceUpdatePortMessage{
		deviceID,
		board.getPort(),
	}
	return &boardMessage
}

func newDeviceUpdateDeleteMessage(deviceID string) *DeviceUpdateDeleteMessage {
	boardMessage := DeviceUpdateDeleteMessage{
		deviceID,
	}
	return &boardMessage
}

func GetMaxFileSize(event Event, c *WebSocketConnection) error {
	return c.sendOutgoingEventMessage(MaxFileSizeMsg, MaxFileSizeMessage{maxFileSize}, false)
}

func SerialConnect(event Event, c *WebSocketConnection) error {
	var msg SerialBaudMessage
	err := json.Unmarshal(event.Payload, &msg)
	if err != nil {
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:      msg.ID,
			Code:    4,
			Comment: err.Error(),
		}, c)
		return err
	}
	board, exists := detector.GetBoardSync(msg.ID)
	if !exists {
		DeviceUpdateDelete(msg.ID, c)
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 2,
		}, c)
		return nil
	}
	if detector.isFake(msg.ID) {
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 3,
		}, c)
		return nil
	}
	updated := board.updatePortName(msg.ID)
	if updated {
		if board.IsConnectedSync() {
			DeviceUpdatePort(msg.ID, board, c)
		} else {
			detector.DeleteBoard(msg.ID)
			DeviceUpdateDelete(msg.ID, c)
			SerialConnectionStatus(DeviceCommentCodeMessage{
				ID:   msg.ID,
				Code: 2,
			}, c)
			return nil
		}
	}
	// плата блокируется!!!
	// не нужно использовать sync функции внутри блока
	board.mu.Lock()
	defer board.mu.Unlock()
	if board.IsFlashBlocked() {
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 5,
		}, c)
		return nil
	}
	if board.isSerialMonitorOpen() {
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 6,
		}, c)
		return nil
	}
	serialPort, err := openSerialPort(board.getSerialPortName(), msg.Baud)
	if err != nil {
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:      msg.ID,
			Code:    1,
			Comment: err.Error(),
		}, c)
		return nil
	}
	SerialConnectionStatus(DeviceCommentCodeMessage{
		ID:   msg.ID,
		Code: 0,
	}, c)
	board.setSerialPortMonitor(serialPort, c, msg.Baud)
	go handleSerial(board, msg.ID, c)
	return nil
}

func SerialConnectionStatus(status DeviceCommentCodeMessage, c *WebSocketConnection) {
	c.sendOutgoingEventMessage(SerialConnectionStatusMsg, status, false)
}

func SerialDisconnect(event Event, c *WebSocketConnection) error {
	var msg SerialDisconnectMessage
	err := json.Unmarshal(event.Payload, &msg)
	if err != nil {
		return err
	}
	board, exists := detector.GetBoardSync(msg.ID)
	// плата блокируется!!!
	// не нужно использовать sync функции внутри блока
	board.mu.Lock()
	defer board.mu.Unlock()
	if exists {
		if board.getSerialMonitorClient() != c {
			SerialConnectionStatus(DeviceCommentCodeMessage{
				ID:   msg.ID,
				Code: 14,
			}, c)
			return nil
		}
		board.closeSerialMonitor()
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 8,
		}, c)
	} else {
		DeviceUpdateDelete(msg.ID, c)
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 2,
		}, c)
	}
	return nil
}

func SerialSend(event Event, c *WebSocketConnection) error {
	var msg SerialMessage
	err := json.Unmarshal(event.Payload, &msg)
	if err != nil {
		SerialSentStatus(DeviceCommentCodeMessage{
			ID:      msg.ID,
			Code:    4,
			Comment: err.Error(),
		}, c)
		return err
	}
	board, exists := detector.GetBoardSync(msg.ID)
	if !exists {
		DeviceUpdateDelete(msg.ID, c)
		SerialSentStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 2,
		}, c)
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 2,
		}, c)
		return nil
	}
	if !board.isSerialMonitorOpenSync() {
		SerialSentStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 3,
		}, c)
		return nil
	}
	updated := board.updatePortName(msg.ID)
	if updated {
		if board.IsConnectedSync() {
			DeviceUpdatePort(msg.ID, board, c)
		} else {
			detector.DeleteBoard(msg.ID)
			DeviceUpdateDelete(msg.ID, c)
			SerialSentStatus(DeviceCommentCodeMessage{
				ID:   msg.ID,
				Code: 2,
			}, c)
			return nil
		}
	}
	if board.getSerialMonitorClientSync() != c {
		SerialSentStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 5,
		}, c)
	}
	// см. handleSerial в serialMonitor.go
	board.serialMonitorWrite <- msg.Msg
	return nil
}

func SerialSentStatus(status DeviceCommentCodeMessage, c *WebSocketConnection) {
	c.sendOutgoingEventMessage(SerialSentStatusMsg, status, false)
}

func SerialChangeBaud(event Event, c *WebSocketConnection) error {
	var msg SerialBaudMessage
	err := json.Unmarshal(event.Payload, &msg)
	if err != nil {
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:      msg.ID,
			Code:    11,
			Comment: err.Error(),
		}, c)
		return err
	}
	board, exists := detector.GetBoardSync(msg.ID)
	if !exists {
		DeviceUpdateDelete(msg.ID, c)
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 2,
		}, c)
		return nil
	}
	if !board.isSerialMonitorOpenSync() {
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 12,
		}, c)
		return nil
	}
	if board.getSerialMonitorClientSync() != c {
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 13,
		}, c)
		return nil
	}
	if msg.Baud == board.getBaudSync() {
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 15,
		}, c)
		return nil
	}
	// см. handleSerial в serialMonitor.go
	board.serialMonitorChangeBaud <- msg.Baud
	return nil
}

func MSPing(event Event, c *WebSocketConnection) error {
	var msg MSPingMessage
	err := json.Unmarshal(event.Payload, &msg)
	if err != nil {
		return err
	}
	board, exists := detector.GetBoardSync(msg.ID)
	if !exists {
		DeviceUpdateDelete(msg.ID, c)
		MSPingResult(msg.ID, 1, c)
		return nil
	}
	updated := board.updatePortName(msg.ID)
	if updated {
		if board.IsConnectedSync() {
			DeviceUpdatePort(msg.ID, board, c)
		} else {
			detector.DeleteBoard(msg.ID)
			DeviceUpdateDelete(msg.ID, c)
			MSPingResult(msg.ID, 1, c)
			return nil
		}
	}
	board.mu.Lock()
	defer board.mu.Unlock()
	portMS, err := ms1.MkSerial(board.getPort())
	if err != nil {
		MSPingResult(msg.ID, 2, c)
		return err
	}
	defer portMS.Close()
	deviceMS := ms1.NewDevice(portMS)
	_, err = deviceMS.Ping()
	if err != nil {
		MSPingResult(msg.ID, 2, c)
		return err
	}
	MSPingResult(msg.ID, 0, c)
	return nil
}

func MSGetAddress(event Event, c *WebSocketConnection) error {
	var msg MSGetAddressMessage
	err := json.Unmarshal(event.Payload, &msg)
	if err != nil {
		return err
	}
	board, exists := detector.GetBoardSync(msg.ID)
	if !exists {
		DeviceUpdateDelete(msg.ID, c)
		MSAddress(msg.ID, 1, "", c)
		return nil
	}
	updated := board.updatePortName(msg.ID)
	if updated {
		if board.IsConnectedSync() {
			DeviceUpdatePort(msg.ID, board, c)
		} else {
			detector.DeleteBoard(msg.ID)
			DeviceUpdateDelete(msg.ID, c)
			MSAddress(msg.ID, 1, "", c)
			return nil
		}
	}
	board.mu.Lock()
	defer board.mu.Unlock()
	portMS, err := ms1.MkSerial(board.getPort())
	if err != nil {
		MSAddress(msg.ID, 2, err.Error(), c)
		return err
	}
	defer portMS.Close()
	deviceMS := ms1.NewDevice(portMS)
	_, err, b := deviceMS.GetId(true, true)
	if err != nil || b == false {
		MSAddress(msg.ID, 2, "Не удалось получить ID устройства. "+err.Error(), c)
		return err
	}
	board.setAddressMS(deviceMS.GetAddress())
	MSAddress(msg.ID, 0, deviceMS.GetAddress(), c)
	return nil
}

func MSPingResult(deviceID string, code int, c *WebSocketConnection) {
	c.sendOutgoingEventMessage(MSPingResultMsg, MSPingResultMessage{
		ID:   deviceID,
		Code: code,
	}, false)
}

func DeviceCommentCode(messageType string, deviceID string, code int, comment string, c *WebSocketConnection) {
	c.sendOutgoingEventMessage(messageType, DeviceCommentCodeMessage{
		ID:      deviceID,
		Code:    code,
		Comment: comment,
	}, false)
}

func MSAddress(deviceID string, code int, comment string, c *WebSocketConnection) {
	DeviceCommentCode(MSAddressMsg, deviceID, code, comment, c)
}

func deviceMessageMakeSync(deviceID string, board *BoardFlashAndSerial) *DeviceMessage {
	board.mu.Lock()
	defer board.mu.Unlock()
	devMes := DeviceMessage{
		ID:         deviceID,
		Name:       board.Type.Name,
		Controller: board.Type.Controller,
		Programmer: board.Type.Programmer,
		SerialID:   board.SerialID,
	}
	if board.portsNum() > 1 {
		devMes.PortNames = board.getPorts()
	} else {
		devMes.PortName = board.getPort()
	}
	return &devMes
}
