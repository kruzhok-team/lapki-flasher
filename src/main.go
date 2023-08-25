package main

import (
	_ "embed"
	"flag"
	"log"
	"net/http"
	"time"
)

//go:embed index.html
var staticPage []byte

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

var detector *Detector

func showJS(w http.ResponseWriter, r *http.Request) {
	w.Write(staticPage)
}

func setArgs() {
	flag.StringVar(&webAddress, "address", "localhost:8080", "адресс для подключения")
	flag.IntVar(&maxMsgSize, "msgSize", 1024, "максмальный размер одного сообщения, передаваемого через веб-сокеты (в байтах)")
	flag.IntVar(&maxFileSize, "fileSize", 2*1024*1024, "максимальный размер файла, загружаемого на сервер (в байтах)")
	flag.IntVar(&maxThreadsPerClient, "thread", 3, "максимальное количество потоков (горутин) на обработку запросов на одного клиента")
	flag.BoolVar(&verbose, "verbose", false, "выводить в консоль подробную информацию")
	getListCooldownSeconds := flag.Int("listCooldown", 2, "минимальное время (в секундах), через которое клиент может снова запросить список устройств, игнорируется, если количество клиентов меньше чем 2")
	updateListTimeSeconds := flag.Int("updateList", 15, "количество секунд между автоматическими обновлениями")
	flag.Parse()
	getListCooldownDuration = time.Second * time.Duration(*getListCooldownSeconds)
	updateListTime = time.Second * time.Duration(*updateListTimeSeconds)
}

func printLog(v ...any) {
	if verbose {
		log.Println(v...)
	}
}

func main() {
	setupOS()
	setArgs()
	log.Printf("Модуль загрузчика запущен со следующими параметрами:\n адрес: %s\n максимальный размер файла: %d\n максимальный размер сообщения: %d\n максимальное количество потоков (горутин) для обработки запросов на одного клиента: %d\n перерыв для запроса списка устройств: %v\n промежуток времени между автоматическими обновлениями: %v\n вывод подробной информации в консоль: %v", webAddress, maxFileSize, maxMsgSize, maxThreadsPerClient, getListCooldownDuration, updateListTime, verbose)

	detector = NewDetector()
	manager := NewWebSocketManager()
	http.HandleFunc("/", showJS)
	http.HandleFunc("/flasher", manager.serveWS)

	log.Fatal(http.ListenAndServe(webAddress, nil))
}
