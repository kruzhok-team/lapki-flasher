package main

import (
	"strconv"
	"time"

	"github.com/albenik/go-serial/v2"
)

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
	//broadcast <- fmt.Sprintf("Подключение к последовательному порту %s со скоростью %d успешно!", currentSettings.Port, currentSettings.BaudRate)
	return serialPort, nil
}

// Чтение, отправка сообщений и изменение скорости передачи последовательного порта
func handleSerial(board *BoardFlashAndSerial, deviceID string, client *WebSocketConnection) {
	defer func() {
		printLog("Serial monitor is closed")
		board.closeSerialMonitorSync()
	}()
	for {
		if client.isClosedChan() || !board.isSerialMonitorOpenSync() {
			return
		}
		if !detector.boardExistsSync(deviceID) {
			DeviceUpdateDelete(deviceID, client)
			SerialConnectionStatus(DeviceCommentCodeMessage{
				ID:   deviceID,
				Code: 2,
			}, client)
			return
		}
		select {
		case baud := <-board.serialMonitorChangeBaud:
			// TODO: можно заменить на configure, но нужно дополнительно протестить, так как при использовании configure в прошлый раз возникла проблема с тем, что не получалось осуществить повторное подключение
			err := board.serialPortMonitor.Close()
			if err != nil {
				SerialConnectionStatus(DeviceCommentCodeMessage{
					ID:      deviceID,
					Code:    9,
					Comment: err.Error(),
				}, client)
				return
			}
			//time.Sleep(time.Second)
			newSerialPort, err := openSerialPort(board.getSerialPortName(), baud)
			if err != nil {
				SerialConnectionStatus(DeviceCommentCodeMessage{
					ID:      deviceID,
					Code:    9,
					Comment: err.Error(),
				}, client)
				return
			}
			board.setSerialPortMonitorSync(newSerialPort, client, baud)
			SerialConnectionStatus(DeviceCommentCodeMessage{
				ID:      deviceID,
				Code:    10,
				Comment: strconv.Itoa(baud),
			}, client)
		case writeMsg := <-board.serialMonitorWrite:
			_, err := board.serialPortMonitor.Write([]byte(writeMsg))
			if err != nil {
				SerialSentStatus(DeviceCommentCodeMessage{
					ID:      deviceID,
					Code:    1,
					Comment: err.Error(),
				}, client)
				SerialConnectionStatus(DeviceCommentCodeMessage{
					ID:   deviceID,
					Code: 1,
				}, client)
				return
			}
			SerialSentStatus(DeviceCommentCodeMessage{
				ID:   deviceID,
				Code: 0,
			}, client)
		default:
			buf := make([]byte, 128)
			bytes, err := board.serialPortMonitor.Read(buf)
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
				}, client)
				return
			}
			if bytes == 0 {
				continue
			}
			err = client.sendOutgoingEventMessage(
				SerialDeviceReadMsg,
				SerialMessage{
					ID:  deviceID,
					Msg: string(buf[:bytes]),
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
