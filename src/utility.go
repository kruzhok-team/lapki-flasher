package main

import (
	"log"
	"os"
	"path/filepath"
)

// exists returns whether the given file or directory exists
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func getAbolutePath(path string) string {
	abspath, err := filepath.Abs(path)
	if err != nil {
		log.Println(err.Error())
		return ""
	}
	return abspath
}
