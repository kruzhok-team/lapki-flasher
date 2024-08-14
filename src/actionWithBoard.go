package main

type BoardAction int

const (
	NOTHING     BoardAction = iota
	ADD         BoardAction = iota
	DELETE      BoardAction = iota
	PORT_UPDATE BoardAction = iota
)

type ActionWithBoard struct {
	board   *BoardFlashAndSerial
	boardID string
	action  BoardAction
}
