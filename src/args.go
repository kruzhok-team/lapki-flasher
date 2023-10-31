package main

import (
	"flag"
	"fmt"
	"log"
	"time"
)

// адрес на котором будет работать этот сервер
var webAddress string

// максмальный размер одного сообщения, передаваемого через веб-сокеты (в байтах)
var maxMsgSize int

// максимальный размер файла, загружаемого на сервер (в байтах)
var maxFileSize int

// максимальное количество потоков (горутин) на обработку запросов на одного клиента
var maxThreadsPerClient int

/*
минимальное время, через которое клиент может снова запросить список устройств;

игнорируется, если количество клиентов меньше чем 2
*/
var getListCooldownDuration time.Duration

// количество времени между автоматическими обновлениями
var updateListTime time.Duration

// выводить в консоль подробную информацию
var verbose bool

// всегда искать устройства и обновлять их список, даже когда ни один клиент не подключён (в основном требуется для тестирования)
var alwaysUpdate bool

// количество ненастоящих, симулируемых устройств, которые будут восприниматься как настоящие, применяется для тестирования
var fakeBoardsNum int

// чтение флагов и происвоение им стандартных значений
func setArgs() {
	flag.StringVar(&webAddress, "address", "localhost:8080", "адресс для подключения")
	flag.IntVar(&maxMsgSize, "msgSize", 1024, "максмальный размер одного сообщения, передаваемого через веб-сокеты (в байтах)")
	flag.IntVar(&maxFileSize, "fileSize", 2*1024*1024, "максимальный размер файла, загружаемого на сервер (в байтах)")
	flag.IntVar(&maxThreadsPerClient, "thread", 3, "максимальное количество потоков (горутин) на обработку запросов на одного клиента")
	flag.IntVar(&fakeBoardsNum, "stub", 0, "количество ненастоящих, симулируемых устройств, которые будут восприниматься как настоящие, применяется для тестирования")
	flag.BoolVar(&verbose, "verbose", false, "выводить в консоль подробную информацию")
	flag.BoolVar(&alwaysUpdate, "alwaysUpdate", false, "всегда искать устройства и обновлять их список")
	getListCooldownSeconds := flag.Int("listCooldown", 2, "минимальное время (в секундах), через которое клиент может снова запросить список устройств, игнорируется, если количество клиентов меньше чем 2")
	updateListTimeSeconds := flag.Int("updateList", 15, "количество секунд между автоматическими обновлениями")
	flag.Parse()
	getListCooldownDuration = time.Second * time.Duration(*getListCooldownSeconds)
	updateListTime = time.Second * time.Duration(*updateListTimeSeconds)
}

// вывод описания всех параметров с их значениями
func printArgsDesc() {
	webAddressStr := fmt.Sprintf("адрес: %s", webAddress)
	maxFileSizeStr := fmt.Sprintf("максимальный размер файла: %d", maxFileSize)
	maxMsgSizeStr := fmt.Sprintf("максимальный размер сообщения: %d", maxMsgSize)
	maxThreadsPerClientStr := fmt.Sprintf("максимальное количество потоков (горутин) для обработки запросов на одного клиента: %d", maxThreadsPerClient)
	getListCooldownDurationStr := fmt.Sprintf("перерыв для запроса списка устройств: %v", getListCooldownDuration)
	updateListTimeStr := fmt.Sprintf("промежуток времени между автоматическими обновлениями: %v", updateListTime)
	verboseStr := fmt.Sprintf("вывод подробной информации в консоль: %v", verbose)
	alwaysUpdateStr := fmt.Sprintf("постоянное обновление списка устройств: %v", alwaysUpdate)
	fakeBoardsNumStr := fmt.Sprintf("количество фальшивых устройств: %d", fakeBoardsNum)
	log.Printf("Модуль загрузчика запущен со следующими параметрами:\n %s\n %s\n %s\n %s\n %s\n %s\n %s\n %s\n %s\n", webAddressStr, maxFileSizeStr, maxMsgSizeStr, maxThreadsPerClientStr, getListCooldownDurationStr, updateListTimeStr, verboseStr, alwaysUpdateStr, fakeBoardsNumStr)
}
