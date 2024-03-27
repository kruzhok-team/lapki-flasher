package main

import (
	"io/ioutil"
	"os"
)

// пишет данные в файл,
type FlashFileWriter struct {
	// размер полученных данных, указывается в байтах
	curSize int
	// необходимый размер файла, указывается в байтах
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

// сохраняет блоки с данными, создаёт временный файл и записывает туда данные
// true = временный файл создан и все данные туда записаны
// возвращает ошибки записи/создании файла, а также если добавление блока превысит максимальный размер файла
func (ff *FlashFileWriter) AddBlock(data []byte) (bool, error) {
	ff.curSize += len(data)
	// добавление этого блока приведёт к превышению указанного размера файла (maxSize)
	if ff.curSize > ff.maxSize {
		return false, ErrFlashLargeBlock
	}
	if ff.tempFile == nil {
		// временный файл для хранения прошивки
		tempFile, err := ioutil.TempFile("", "upload-*.hex")
		if err != nil {
			return false, err
		}
		ff.tempFile = tempFile
	}
	ff.tempFile.Write(data)
	// не все блоки получены, файл не создаётся
	if ff.curSize < ff.maxSize {
		return false, nil
	}
	// получены все блоки
	ff.tempFile.Close()
	return true, nil
}

// удаление файла и данных
func (ff *FlashFileWriter) Clear() {
	ff.maxSize = 0
	ff.curSize = 0
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
