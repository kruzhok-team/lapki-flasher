package main

import "errors"

// сообщения-ошибки для клиента
var (
	// неизвестный тип сообщения
	ErrEventNotSupported = errors.New("event-not-supported")
	// предыдущая операция прошивки ещё не завершена
	ErrFlashNotFinished = errors.New("flash-not-finish")
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
)

// TODO
func errorHandler(err error) {

}
