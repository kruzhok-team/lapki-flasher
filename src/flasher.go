package main

import (
	"sync"
)

var flasherSync sync.Mutex

func handleFlashResult(flashOutput string, flashError error) (result string) {
	if flashError != nil {
		result = flashError.Error()
		if flashOutput != "" {
			result += "\n" + flashOutput
		}
	} else {
		result = flashOutput
	}
	return
}
