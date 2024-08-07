package main

import (
	"time"

	"github.com/tarm/serial"
)

// Открываем порт заново, если он был закрыт
func openSerialPort(port string, baudRate int) (*serial.Port, error) {
	c := &serial.Config{Name: port, Baud: baudRate, ReadTimeout: 5 * time.Second}
	var err error
	serialPort, err := serial.OpenPort(c)
	if err != nil {
		// Ошибка: не удалось открыть последовательный порт. Проверьте настройки и переподключитесь к порту.
		return nil, err
	}
	//broadcast <- fmt.Sprintf("Подключение к последовательному порту %s со скоростью %d успешно!", currentSettings.Port, currentSettings.BaudRate)
	return serialPort, nil
}

// Получаем ответ из последовательного порта
func readFromSerial(board *BoardFlashAndSerial, deviceID string, client *WebSocketConnection) {
	for {
		// Читаем до символа новой строки
		if client.isClosedChan() || !board.isSerialMonitorOpen() {
			break
		}
		if !detector.boardExists(deviceID) {
			DeviceUpdateDelete(deviceID, client)
			SerialConnectionStatus(SerialStatusMessage{
				ID:   deviceID,
				Code: 2,
			}, client)
			break
		}
		buf := make([]byte, 128)
		bytes, err := board.serialPortMonitor.Read(buf)
		if bytes == 0 {
			continue
		}
		if err != nil {
			// Ошибка при чтении из последовательного порта
			SerialConnectionStatus(SerialStatusMessage{
				ID:      deviceID,
				Code:    7,
				Comment: err.Error(),
			}, client)
			break
		}
		// printLog(len(buf[:bytes]), len(string(buf[:bytes])))
		str := string(buf[:bytes])
		printLog(buf[bytes-1], str[bytes-1])
		err = client.sendOutgoingEventMessage(
			SerialDeviceReadMsg,
			SerialMessage{
				ID:  deviceID,
				Msg: string(buf[:bytes]),
			},
			false,
		)
		if err != nil {
			break
		}
		// Добавляем небольшую задержку перед чтением нового сообщения
		time.Sleep(100 * time.Millisecond)
	}
	printLog("Serial monitor is closed")
	board.closeSerialMonitor()
}

// Отправление сообщения от клиента в последовательный порт
func writeToSerial(serialPort *serial.Port, msg string) error {
	_, err := serialPort.Write([]byte(msg))
	return err
}
