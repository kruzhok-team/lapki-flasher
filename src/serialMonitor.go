package main

import (
	"bufio"
	"strings"
	"time"

	"github.com/tarm/serial"
)

// Открываем порт заново, если он был закрыт
func openSerialPort(port string, baudRate int) (*serial.Port, error) {
	c := &serial.Config{Name: port, Baud: baudRate}
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
func readFromSerial(serialPort *serial.Port, deviceID string, client *WebSocketConnection) error {
	reader := bufio.NewReader(serialPort)
	for {
		// Читаем до символа новой строки
		receivedMsg, err := reader.ReadString('\n')
		if err != nil {
			// Ошибка при чтении из последовательного порта
			SerialConnectionStatus(SerialStatusMessage{
				ID:      deviceID,
				Code:    7,
				Comment: err.Error(),
			}, client)
			return err
		}
		// Удаляем пробельные символы
		receivedMsg = strings.TrimSpace(receivedMsg)
		if receivedMsg != "" {
			// Отправляем сообщение клиенту
			client.sendOutgoingEventMessage(
				SerialDeviceReadMsg,
				SerialMessage{
					ID:  deviceID,
					Msg: receivedMsg,
				},
				false,
			)
		}
		// Добавляем небольшую задержку перед чтением нового сообщения
		time.Sleep(100 * time.Millisecond)
	}
}

// Отправление сообщения от клиента в последовательный порт
func writeToSerial(serialPort *serial.Port, msg string) error {
	_, err := serialPort.Write([]byte(msg))
	return err
}
