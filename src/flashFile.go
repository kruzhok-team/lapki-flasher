package main

import (
	"fmt"
	"io/ioutil"
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
	filePath string
}

func newFlashFileWriter() *FlashFileWriter {
	var writer FlashFileWriter
	writer.maxSize = 0
	writer.curSize = 0
	writer.blocks = blocksArray{}
	writer.filePath = ""
	return &writer
}

// начать новую запись
func (ff *FlashFileWriter) Start(fileSize int) {
	ff.Clear()
	fmt.Println(fileSize)
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
	if err != nil {
		return false, err
	}
	defer func() {
		tempFile.Close()
	}()

	// сортируем по индексам в возрастающем порядке
	sort.Sort(blocksArray(ff.blocks))

	// записываем данные в файл
	for _, block := range ff.blocks {
		tempFile.Write(block.Data)
	}

	return true, nil
}

// TODO: удаление файла и данных
func (ff *FlashFileWriter) Clear() {

}

func (ff *FlashFileWriter) GetFilePath() string {
	return ff.filePath
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
