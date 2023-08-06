package main

import (
	"errors"
)

// сообщения-ошибки для клиента
var (
	// неизвестный тип сообщения
	ErrEventNotSupported = errors.New("event-not-supported")
	// предыдущая операция прошивки ещё не завершена
	ErrFlashNotFinished = errors.New("flash-not-finished")
	// прошивка не началась (не была отправлена команда flash-start от клиента)
	ErrFlashNotStarted = errors.New("flash-not-started")
	// устройство с таким ID отсутствует в списке
	ErrFlashWrongID = errors.New("flash-wrong-id")
	// прошивка не удалась, потому что устройство отключилось
	ErrFlashDisconnected = errors.New("flash-disconnected")
	// устройство заблокировано другим пользователем для прошивки
	ErrFlashBlocked = errors.New("flash-blocked")
	// не получилось добавить блок данных (flash-block), так как его размер слишком большой
	// либо отправлен неправильный блок, либо был указан неправильный размер файла
	ErrFlashLargeBlock = errors.New("flash-large-block")
	// указанный размер файла превышает максимально допустимый размер файла, установленный сервером (MAX_FILE_SIZE)
	ErrFlashLargeFile = errors.New("flash-large-file")
	// ошибка от avrdude
	ErrAvrdude = errors.New("flash-avrdude-error")
	// ошибка при чтение JSON-объекта
	ErrUnmarshal = errors.New("unmarshal-err")
	// прошлый запрос get-list находится в cooldown
	ErrGetListCoolDown = errors.New("get-list-cooldown")
	// Слишком много запросов от клиента всё ещё находятся в обработке, сервер возвращает сообщение обработно, оно будет записано в payload
	ErrWaitingMessagesLimit = errors.New("waiting-message-limit")
	// аналогично ErrWaitingMessagesLimit, но для бинарных данных, не возвращает данные обратно клиенту
	ErrWaitingBinaryMessagesLimit = errors.New("waiting-binary-message-limit")
)

func errorHandler(err error, c *WebSocketConnection) {
	if err == nil {
		return
	}
	msgType := err.Error()
	var payload any
	switch err {
	case ErrFlashWrongID, ErrFlashDisconnected, ErrFlashBlocked, ErrFlashLargeFile, ErrFlashLargeBlock:
		c.StopFlashing()
	case ErrAvrdude:
		c.StopFlashing()
		payload = c.avrMsg
		defer func() {
			c.avrMsg = ""
		}()
	}
	c.sentOutgoingEventMessage(msgType, payload, false)
}
