package main

import (
	"io/ioutil"
	"os"
	"sort"
)

type blocksArray []*FlashBlockMessage

// пишет данные в файл,
type FlashFileWriter struct {
	blocks blocksArray
	// размер указывается в байтах
	curSize int
	// необходимый размер файла
	maxSize  int
	tempFile *os.File
}

func newFlashFileWriter() *FlashFileWriter {
	var writer FlashFileWriter
	writer.Clear()
	return &writer
}

// начать новую запись
func (ff *FlashFileWriter) Start(fileSize int) {
	ff.Clear()
	ff.maxSize = fileSize
}

// сохраняет блоки с данными, создаёт временный файл и записывает туда данные, после получения всех блоков
// true = временный файл создан и все данные туда записаны
// возвращает ошибки записи/создании файла, а также если добавление блока превысит максимальный размер файла
func (ff *FlashFileWriter) AddBlock(block *FlashBlockMessage) (bool, error) {
	ff.curSize += len(block.Data)
	// добавление этого блока приведёт к превышению указанного размера файла (maxSize)
	if ff.curSize > ff.maxSize {
		return false, ErrFlashLargeBlock
	}
	ff.blocks = append(ff.blocks, block)
	// не все блоки получены, файл не создаётся
	if ff.curSize < ff.maxSize {
		return false, nil
	}
	// получены все блоки

	// временный файл для хранения прошивки
	tempFile, err := ioutil.TempFile("", "upload-*.hex")
	ff.tempFile = tempFile
	//fmt.Println("Temp File Name", ff.tempFile.Name())
	if err != nil {
		return false, err
	}
	defer func() {
		ff.tempFile.Close()
	}()

	// сортируем по индексам в возрастающем порядке
	sort.Sort(blocksArray(ff.blocks))

	// записываем данные в файл
	for _, block := range ff.blocks {
		ff.tempFile.Write(block.Data)
	}
	return true, nil
}

// TODO: удаление файла и данных
func (ff *FlashFileWriter) Clear() {
	ff.maxSize = 0
	ff.curSize = 0
	ff.blocks = blocksArray{}
	if ff.tempFile != nil {
		ff.tempFile.Close()
		os.Remove(ff.tempFile.Name())
	}
	ff.tempFile = nil
}

// возвращает пустую строку, если временного файла не существует
func (ff *FlashFileWriter) GetFilePath() string {
	if ff.tempFile != nil {
		return ff.tempFile.Name()
	}
	return ""
}

// методы sort.Interface для blocksArray

func (blocks blocksArray) Len() int {
	return len(blocks)
}

func (blocks blocksArray) Swap(i, j int) {
	blocks[i], blocks[j] = blocks[j], blocks[i]
}

func (blocks blocksArray) Less(i, j int) bool {
	return blocks[i].BlockID < blocks[j].BlockID
}
