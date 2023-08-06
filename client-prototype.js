// Общий вид для всех сообщений, как от клиента так и от сервера, единственное исключение "бинарные данные файла", они не отправляются через JSON
class Event {
    constructor(type, payload) {
        // Тип сообщения (flash-start, get-list и т.д.)
        this.type = type;
        // Параметры сообщения, не все сообщения обязаны иметь параметры
        this.payload = payload;
    }
}
// описание устройства
class Device {
    constructor(id, name, controller, programmer, port){
        this.deviceID = id;
        this.name = name;
        this.controller = controller;
        this.programmer = programmer;
        this.portName = port; 
    }
}
// запрос на прошивку
class FlashStart {
    constructor(id, fileSize){
        this.deviceID = id;
        this.fileSize = fileSize;
    }
}
// оповещение о том, что данное устройство заблокировано для прошивки
class FlashBlock{
    constructor(blockID, data){
        this.blockID = blockID;
        this.data = data;
    }
}

const FLASH_NEXT_BLOCK = "flash-next-block"
const FLASH_DONE = "flash-done"