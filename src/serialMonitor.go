package main

import (
	"strconv"
	"time"

	"github.com/albenik/go-serial/v2"
)

type SerialMonitor struct {
	// порт на котром открыт монитор порта, nil значит, что монитор порта закрыт
	Port *serial.Port
	// канал для оповещения о том, что следует сменить бод
	ChangeBaud chan int
	// текущее значение бод
	Baud int
	// клиент, который открыл монитор порта этого устройства
	Client *WebSocketConnection
	// открыт ли монитор порта
	Open bool
	// канал для передачи на устройство
	Write chan []byte
}

func (serialMonitor *SerialMonitor) set(serialPort *serial.Port, serialClient *WebSocketConnection, baud int) {
	serialMonitor.Port = serialPort
	serialMonitor.Client = serialClient
	serialMonitor.ChangeBaud = make(chan int)
	serialMonitor.Baud = baud
	serialMonitor.Open = true
	serialMonitor.Write = make(chan []byte)
}

func (serialMonitor *SerialMonitor) isOpen() bool {
	return serialMonitor.Port != nil && serialMonitor.Open
}

func (serialMonitor *SerialMonitor) close() {
	if serialMonitor.Port == nil {
		return
	}
	if err := serialMonitor.Port.Close(); err != nil {
		printLog(err.Error())
	}
	serialMonitor.Open = false
}

// Открываем порт заново, если он был закрыт
func openSerialPort(port string, baudRate int) (*serial.Port, error) {
	// TODO: вынести настройку ReadTimeout/WriteTimeout во флаги
	// время для таймаутов указано в мс
	// не стоит задавать слишком большой таймаут, чтение, запись и смена скорости передачи происходит в одном потоке, слишком большой таймаут может замедлить работу с портом
	serialPort, err := serial.Open(
		port,
		serial.WithBaudrate(baudRate),
		serial.WithReadTimeout(100),
		serial.WithWriteTimeout(100),
	)
	if err != nil {
		// Ошибка: не удалось открыть последовательный порт. Проверьте настройки и переподключитесь к порту.
		return nil, err
	}
	return serialPort, nil
}

// Чтение, отправка сообщений и изменение скорости передачи последовательного порта
func handleSerial(board *Device, deviceID string) {
	defer func() {
		printLog("Serial monitor is closed")
		board.Mu.Lock()
		board.SerialMonitor.close()
		board.Mu.Unlock()
	}()
	for {
		if board.SerialMonitor.Client.isClosedChan() || !board.isSerialMonitorOpenSync() {
			return
		}
		if !detector.boardExistsSync(deviceID) {
			DeviceUpdateDelete(deviceID, board.SerialMonitor.Client)
			SerialConnectionStatus(DeviceCommentCodeMessage{
				ID:   deviceID,
				Code: 2,
			}, board.SerialMonitor.Client)
			return
		}
		select {
		case baud := <-board.SerialMonitor.ChangeBaud:
			// TODO: можно заменить на configure, но нужно дополнительно протестить, так как при использовании configure в прошлый раз возникла проблема с тем, что не получалось осуществить повторное подключение
			err := board.SerialMonitor.Port.Close()
			if err != nil {
				SerialConnectionStatus(DeviceCommentCodeMessage{
					ID:      deviceID,
					Code:    9,
					Comment: err.Error(),
				}, board.SerialMonitor.Client)
				return
			}
			//time.Sleep(time.Second)
			newSerialPort, err := openSerialPort(board.Board.GetSerialPort(), baud)
			if err != nil {
				SerialConnectionStatus(DeviceCommentCodeMessage{
					ID:      deviceID,
					Code:    9,
					Comment: err.Error(),
				}, board.SerialMonitor.Client)
				return
			}
			board.Mu.Lock()
			board.SerialMonitor.set(newSerialPort, board.SerialMonitor.Client, baud)
			board.Mu.Unlock()
			SerialConnectionStatus(DeviceCommentCodeMessage{
				ID:      deviceID,
				Code:    10,
				Comment: strconv.Itoa(baud),
			}, board.SerialMonitor.Client)
		case writeMsg := <-board.SerialMonitor.Write:
			_, err := board.SerialMonitor.Port.Write(writeMsg)
			if err != nil {
				SerialSentStatus(DeviceCommentCodeMessage{
					ID:      deviceID,
					Code:    1,
					Comment: err.Error(),
				}, board.SerialMonitor.Client)
				SerialConnectionStatus(DeviceCommentCodeMessage{
					ID:   deviceID,
					Code: 1,
				}, board.SerialMonitor.Client)
				return
			}
			SerialSentStatus(DeviceCommentCodeMessage{
				ID:   deviceID,
				Code: 0,
			}, board.SerialMonitor.Client)
		default:
			buf := make([]byte, 128)
			bytes, err := board.SerialMonitor.Port.Read(buf)
			if err != nil {
				// если ошибка произошла из-за того, монитор порта закрылся, то
				// игнорируем ошибку
				if !board.isSerialMonitorOpenSync() {
					return
				}
				// Ошибка при чтении из последовательного порта
				SerialConnectionStatus(DeviceCommentCodeMessage{
					ID:      deviceID,
					Code:    7,
					Comment: err.Error(),
				}, board.SerialMonitor.Client)
				return
			}
			if bytes == 0 {
				continue
			}
			err = board.SerialMonitor.Client.sendOutgoingEventMessage(
				SerialDeviceReadMsg,
				SerialMessage{
					ID:  deviceID,
					Msg: buf[:bytes],
				},
				false,
			)
			if err != nil {
				return
			}
			// Добавляем небольшую задержку перед чтением нового сообщения
			time.Sleep(100 * time.Millisecond)
		}
	}
}
