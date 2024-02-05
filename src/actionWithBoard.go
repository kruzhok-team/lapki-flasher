package main

type BoardAction int

const (
	NOTHING     BoardAction = iota
	ADD         BoardAction = iota
	DELETE      BoardAction = iota
	PORT_UPDATE BoardAction = iota
)

type ActionWithBoard struct {
	board   *BoardToFlash
	boardID string
	action  BoardAction
}
