// обработка и отправка сообщений
package main

import (
	"encoding/json"
	"log"
	"strings"
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
	ID         string `json:"deviceID"`
	Name       string `json:"name,omitempty"`
	Controller string `json:"controller,omitempty"`
	Programmer string `json:"programmer,omitempty"`
	PortName   string `json:"portName,omitempty"`
	SerialID   string `json:"serialID,omitempty"`
}

type MSDeviceMessage struct {
	ID        string    `json:"deviceID"`
	Name      string    `json:"name,omitempty"`
	PortNames [4]string `json:"portNames,omitempty"`
}

// тип данных для flash-start (для arduino-подобных устройств)
type FlashStartMessage struct {
	ID       string `json:"deviceID"`
	FileSize int    `json:"fileSize"` // размер прошивки
}

// тип данных для ms-bin-start (для МС-ТЮК)
type MSBinStartMessage struct {
	ID           string `json:"deviceID"`
	FileSize     int    `json:"fileSize"`     // размер прошивки
	Address      string `json:"address"`      // киберген
	Verification bool   `json:"verification"` // если true, то загрузчик потратит дополнительное время на проверку прошивки
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

type DeviceCodeMessage struct {
	ID   string `json:"deviceID"`
	Code int    `json:"code"`
}

// тип данных для serial-device-read и serial-send
type SerialMessage struct {
	ID  string `json:"deviceID"`
	Msg string `json:"msg"`
}

type MSAddressMessage struct {
	ID      string `json:"deviceID"`
	Address string `json:"address"`
}

type MSGetAddressMessage struct {
	ID string `json:"deviceID"`
}

type DeviceIdMessage struct {
	ID string `json:"deviceID"`
}

type MSMetaDataMessage struct {
	ID            string `json:"deviceID"`
	RefBlHw       string `json:"RefBlHw"`       // Описывает физическое окружение контроллера (плату)
	RefBlFw       string `json:"RefBlFw"`       // Указывает на версию прошивки загрузчика
	RefBlUserCode string `json:"RefBlUserCode"` //
	RefBlChip     string `json:"RefBlChip"`     // Указывает на контроллер, здесь то, что нужно для компиляции прошивки
	RefBlProtocol string `json:"RefBlProtocol"` // Описывает возможности протокола загрузчика
	RefCgHw       string `json:"RefCgHw"`       // Указывает на аппаратное исполнение
	RefCgFw       string `json:"RefCgFw"`       // Указывает на версию прошивки кибергена
	RefCgProtocol string `json:"RefCgProtocol"` // Указывает на возможности протокола кибергена
	MSType        string `json:"type"`          // тип устройства (определяется по RefBlHw)
}

type FlashBacktrackMsMessage struct {
	UploadStage string `json:"UploadStage"`

	NoPacks bool `json:"NoPacks"`

	CurPack    uint16 `json:"CurPack"`
	TotalPacks uint16 `json:"TotalPacks"`
}

// типы сообщений (событий)
const (
	// запрос на получение списка всех устройств
	GetListMsg = "get-list"
	// описание ардуино подобного устройства
	DeviceMsg = "device"
	// описание МС-ТЮК
	MSDeviceMsg = "ms-device"
	// запрос на прошивку устройства
	FlashStartMsg = "flash-start"
	// прошивка прошла успешна
	FlashDoneMsg = "flash-done"
	// запрос на следующий блок бинарных данных
	FlashNextBlockMsg = "flash-next-block"
	// сообщение, для отметки бинарных данных загружаемого файла прошивки, прикрепляется сервером к сообщению после получения данных бинарного типа
	FlashBinaryBlockMsg = "flash-block"
	// обратная связь от программы загрузки прошивки МС-ТЮК
	FlashBackTrackMs = "flash-backtrack-ms"
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
	// сброс МС-ТЮК
	MSResetMsg = "ms-reset"
	// результат выполнения сброса МС-ТЮК
	MSResetResultMsg = "ms-reset-result"
	// запрос на получение метаданных МС-ТЮК
	MSGetMetaDataMsg = "ms-get-meta-data"
	// сообщение с метаданными, предназначенными для клиента
	MSMetaDataMsg = "ms-meta-data"
	// сообщение в случае, если не удалось извлечь метаданные по запросу клиента
	MSMetaDataErrorMsg = "ms-meta-data-error"
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
func SendDevice(deviceID string, board *Device, toAll bool, c *WebSocketConnection) error {
	err := c.sendOutgoingEventMessage(board.Board.GetWebMessageType(), board.Board.GetWebMessage(board.Name, deviceID), toAll)
	if err != nil {
		printLog("device() error:", err.Error())
	}
	return err
}

// сообщение о том, что порт обновлён
func DeviceUpdatePort(deviceID string, board *Device, c *WebSocketConnection) {
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
	var deviceID string
	var fileSize int
	var address string    // адрес, только для МС-ТЮК
	var verification bool // верификация, только для МС-ТЮК
	if event.Type == FlashStartMsg {
		var msg FlashStartMessage
		err := json.Unmarshal(event.Payload, &msg)
		if err != nil {
			return err
		}
		deviceID = msg.ID
		fileSize = msg.FileSize
	} else {
		var msg MSBinStartMessage
		err := json.Unmarshal(event.Payload, &msg)
		if err != nil {
			return err
		}
		deviceID = msg.ID
		fileSize = msg.FileSize
		address = msg.Address
		verification = msg.Verification
	}
	if fileSize < 1 {
		return nil
	}
	if fileSize > maxFileSize {
		return ErrFlashLargeFile
	}
	dev, exists := detector.GetBoardSync(deviceID)
	if !exists {
		return ErrFlashWrongID
	}
	// плата блокируется!!!
	// не нужно использовать sync функции внутри блока
	dev.Mu.Lock()
	defer dev.Mu.Unlock()
	updated := dev.Board.Update()
	if updated {
		if dev.Board.IsConnected() {
			DeviceUpdatePort(deviceID, dev, c)
		} else {
			detector.DeleteBoard(deviceID)
			DeviceUpdateDelete(deviceID, c)
			return ErrFlashDisconnected
		}
	}
	if dev.IsFlashBlocked() {
		return ErrFlashBlocked
	}
	boardToFlashName := strings.ToLower(dev.Name)
	for _, boardName := range notSupportedBoards {
		if boardToFlashName == strings.ToLower(boardName) {
			c.sendOutgoingEventMessage(ErrNotSupported.Error(), boardName, false)
			return nil
		}
	}
	// расширение для файла прошивки
	var ext string
	switch dev.Board.(type) {
	case *Arduino:
		if dev.SerialMonitor.isOpen() {
			return ErrFlashOpenSerialMonitor
		}
		if event.Type == FlashStartMsg {
			ext = "hex"
		} else {
			// TODO
		}
	case *MS1:
		if event.Type == MSBinStartMsg {
			ext = "bin"
			if address != "" {
				dev.Board.(*MS1).address = address
			}
			dev.Board.(*MS1).verify = verification
		} else {
			// TODO
		}
	}
	// блокировка устройства и клиента для прошивки, необходимо разблокировать после завершения прошивки
	c.FlashingBoard = dev
	c.FlashingBoard.SetLock(true)
	c.FileWriter.Start(fileSize, ext)

	FlashNextBlock(c)
	return nil
}

func LogSend(client *WebSocketConnection, logger chan any) {
	if client.FlashingBoard == nil || logger == nil {
		return
	}
	switch client.FlashingBoard.Board.(type) {
	case *MS1:
		for log := range logger {
			client.sendOutgoingEventMessage(FlashBackTrackMs, log, false)
		}
		printLog("firmware logging is over")
	}
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
		logger := make(chan any)
		go LogSend(c, logger)
		avrMsg, err := c.FlashingBoard.Board.Flash(c.FileWriter.GetFilePath(), logger)
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

func newDeviceUpdatePortMessage(dev *Device, deviceID string) *DeviceUpdatePortMessage {
	boardMessage := DeviceUpdatePortMessage{
		deviceID,
		dev.Board.(*Arduino).portName,
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
	dev, exists := detector.GetBoardSync(msg.ID)
	if !exists {
		DeviceUpdateDelete(msg.ID, c)
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 2,
		}, c)
		return nil
	}
	if dev.isFake() {
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 3,
		}, c)
		return nil
	}
	// плата блокируется!!!
	// не нужно использовать sync функции внутри блока
	dev.Mu.Lock()
	defer dev.Mu.Unlock()
	updated := dev.Board.Update()
	if updated {
		if dev.Board.IsConnected() {
			DeviceUpdatePort(msg.ID, dev, c)
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
	if _, isArduino := dev.Board.(*Arduino); isArduino && dev.IsFlashBlocked() {
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 5,
		}, c)
		return nil
	}
	if dev.SerialMonitor.isOpen() {
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 6,
		}, c)
		return nil
	}
	serialPort, err := openSerialPort(dev.Board.GetSerialPort(), msg.Baud)
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
	dev.SerialMonitor.set(serialPort, c, msg.Baud)
	go handleSerial(dev, msg.ID)
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
	board.Mu.Lock()
	defer board.Mu.Unlock()
	if exists {
		if board.SerialMonitor.Client != c {
			SerialConnectionStatus(DeviceCommentCodeMessage{
				ID:   msg.ID,
				Code: 14,
			}, c)
			return nil
		}
		board.SerialMonitor.close()
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
	dev, exists := detector.GetBoardSync(msg.ID)
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
	if !dev.isSerialMonitorOpenSync() {
		SerialSentStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 3,
		}, c)
		return nil
	}
	updated := dev.updateSync()
	if updated {
		if dev.isConnectedSync() {
			DeviceUpdatePort(msg.ID, dev, c)
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
	if dev.SerialMonitor.Client != c {
		SerialSentStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 5,
		}, c)
	}
	// см. handleSerial в serialMonitor.go
	dev.SerialMonitor.Write <- msg.Msg
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
	dev, exists := detector.GetBoardSync(msg.ID)
	if !exists {
		DeviceUpdateDelete(msg.ID, c)
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 2,
		}, c)
		return nil
	}
	if !dev.isConnectedSync() {
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 12,
		}, c)
		return nil
	}
	if dev.getSerialMonitorClientSync() != c {
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 13,
		}, c)
		return nil
	}
	if msg.Baud == dev.getSerialMonitorBaudSync() {
		SerialConnectionStatus(DeviceCommentCodeMessage{
			ID:   msg.ID,
			Code: 15,
		}, c)
		return nil
	}
	// см. handleSerial в serialMonitor.go
	dev.SerialMonitor.ChangeBaud <- msg.Baud
	return nil
}

func MSPing(event Event, c *WebSocketConnection) error {
	var msg MSAddressMessage
	err := json.Unmarshal(event.Payload, &msg)
	if err != nil {
		MSPingResult(msg.ID, 4, err.Error(), c)
		return err
	}
	dev, exists := detector.GetBoardSync(msg.ID)
	if !exists {
		DeviceUpdateDelete(msg.ID, c)
		MSPingResult(msg.ID, 1, "", c)
		return nil
	}
	dev.Mu.Lock()
	defer dev.Mu.Unlock()
	board, isMS1 := dev.Board.(*MS1)
	if !isMS1 {
		MSPingResult(msg.ID, 3, "", c)
		return nil
	}
	updated := board.Update()
	if updated {
		if dev.Board.IsConnected() {
			// TODO
		} else {
			detector.DeleteBoard(msg.ID)
			DeviceUpdateDelete(msg.ID, c)
			MSPingResult(msg.ID, 1, "", c)
			return nil
		}
	}
	board.address = msg.Address
	err = board.ping()
	if err != nil {
		MSPingResult(msg.ID, 2, err.Error(), c)
		return err
	}
	MSPingResult(msg.ID, 0, "", c)
	return nil
}

func MSGetAddress(event Event, c *WebSocketConnection) error {
	var msg MSGetAddressMessage
	err := json.Unmarshal(event.Payload, &msg)
	if err != nil {
		MSAddressSend(msg.ID, 4, err.Error(), c)
		return err
	}
	dev, exists := detector.GetBoardSync(msg.ID)
	if !exists {
		DeviceUpdateDelete(msg.ID, c)
		MSAddressSend(msg.ID, 1, "", c)
		return nil
	}
	dev.Mu.Lock()
	defer dev.Mu.Unlock()
	board, isMS1 := dev.Board.(*MS1)
	if !isMS1 {
		MSAddressSend(msg.ID, 3, "", c)
		return nil
	}
	updated := board.Update()
	if updated {
		if board.IsConnected() {
			// TODO
		} else {
			detector.DeleteBoard(msg.ID)
			DeviceUpdateDelete(msg.ID, c)
			MSAddressSend(msg.ID, 1, "", c)
			return nil
		}
	}
	address, err := board.getAddress()
	if err != nil {
		MSAddressSend(msg.ID, 2, err.Error(), c)
		return err
	}
	MSAddressSend(msg.ID, 0, address, c)
	return nil
}

func MSPingResult(deviceID string, code int, comment string, c *WebSocketConnection) {
	DeviceCommentCode(MSPingResultMsg, deviceID, code, comment, c)
}

func DeviceCommentCode(messageType string, deviceID string, code int, comment string, c *WebSocketConnection) {
	c.sendOutgoingEventMessage(messageType, DeviceCommentCodeMessage{
		ID:      deviceID,
		Code:    code,
		Comment: comment,
	}, false)
}

func MSAddressSend(deviceID string, code int, comment string, c *WebSocketConnection) {
	DeviceCommentCode(MSAddressMsg, deviceID, code, comment, c)
}

func MSReset(event Event, c *WebSocketConnection) error {
	var msg MSAddressMessage
	err := json.Unmarshal(event.Payload, &msg)
	if err != nil {
		MSResetSend(msg.ID, 4, err.Error(), c)
		return err
	}
	dev, exists := detector.GetBoardSync(msg.ID)
	if !exists {
		DeviceUpdateDelete(msg.ID, c)
		MSResetSend(msg.ID, 1, "", c)
		return nil
	}
	dev.Mu.Lock()
	defer dev.Mu.Unlock()
	board, isMS1 := dev.Board.(*MS1)
	if !isMS1 {
		MSResetSend(msg.ID, 3, "", c)
		return nil
	}
	updated := board.Update()
	if updated {
		if board.IsConnected() {
			// TODO
		} else {
			detector.DeleteBoard(msg.ID)
			DeviceUpdateDelete(msg.ID, c)
			MSResetSend(msg.ID, 1, "", c)
			return nil
		}
	}
	board.address = msg.Address
	err = board.reset()
	if err != nil {
		MSResetSend(msg.ID, 2, err.Error(), c)
		return err
	}
	MSResetSend(msg.ID, 0, "", c)
	return nil
}

func MSResetSend(deviceID string, code int, comment string, client *WebSocketConnection) {
	DeviceCommentCode(MSResetResultMsg, deviceID, code, comment, client)
}

const (
	META_ERROR        = 1
	META_NO_DEVICE    = 2
	META_WRONG_DEVICE = 3
	META_JSON_ERROR   = 4
)

func MSMetaDataError(deviceID string, code int, comment string, client *WebSocketConnection) {
	DeviceCommentCode(MSMetaDataErrorMsg, deviceID, code, comment, client)
}

func MSGetMetaData(event Event, c *WebSocketConnection) error {
	var msg MSAddressMessage
	err := json.Unmarshal(event.Payload, &msg)
	if err != nil {
		MSMetaDataError(msg.ID, META_JSON_ERROR, err.Error(), c)
		return err
	}
	dev, exists := detector.GetBoardSync(msg.ID)
	if !exists {
		DeviceUpdateDelete(msg.ID, c)
		MSMetaDataError(msg.ID, META_NO_DEVICE, "", c)
		return nil
	}
	dev.Mu.Lock()
	defer dev.Mu.Unlock()
	board, isMS1 := dev.Board.(*MS1)
	if !isMS1 {
		MSMetaDataError(msg.ID, META_WRONG_DEVICE, "", c)
		return nil
	}
	updated := board.Update()
	if updated {
		if board.IsConnected() {
			// TODO
		} else {
			detector.DeleteBoard(msg.ID)
			DeviceUpdateDelete(msg.ID, c)
			MSMetaDataError(msg.ID, META_NO_DEVICE, "", c)
			return nil
		}
	}
	board.address = msg.Address
	meta, err := board.getMetaData()
	if err != nil {
		MSMetaDataError(msg.ID, META_ERROR, err.Error(), c)
		return err
	}
	c.sendOutgoingEventMessage(MSMetaDataMsg, MSMetaDataMessage{
		ID:            msg.ID,
		RefBlHw:       meta.RefBlHw,
		RefBlFw:       meta.RefBlFw,
		RefBlUserCode: meta.RefBlUserCode,
		RefBlChip:     meta.RefBlChip,
		RefBlProtocol: meta.RefBlProtocol,
		RefCgHw:       meta.RefCgHw,
		RefCgFw:       meta.RefCgFw,
		RefCgProtocol: meta.RefCgProtocol,
		MSType:        getMSType(meta.RefBlHw),
	}, false)
	return nil
}
