package main

// пишет данные в файл,
type DataTransmission struct {
	curByte   int
	bytes     []byte
	blockSize int
	arraySize int
}

func newDataTransmission() *DataTransmission {
	var transmission DataTransmission
	transmission.Clear()
	return &transmission
}

func (transmission *DataTransmission) Clear() {
	transmission.blockSize = 0
	transmission.bytes = []byte{}
	transmission.curByte = 0
	transmission.arraySize = 0
}

func (trasmission *DataTransmission) set(bytes []byte, blockSize int) {
	trasmission.curByte = 0
	trasmission.bytes = bytes
	trasmission.arraySize = len(bytes)
	trasmission.blockSize = blockSize
}

func (transmission *DataTransmission) popBlock() []byte {
	startIndex := transmission.curByte
	endIndex := min(startIndex+transmission.blockSize, transmission.arraySize)
	transmission.curByte = endIndex
	return transmission.bytes[startIndex:endIndex]
}

func (transmission *DataTransmission) isFinish() bool {
	return transmission.curByte >= transmission.arraySize
}
