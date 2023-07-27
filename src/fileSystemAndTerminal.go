package main

import (
	"fmt"
	"os"
	"os/exec"
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
		fmt.Println(err.Error())
		return ""
	}
	return abspath
}

// выполнение консольной команды с обработкой ошибок и возвращением stdout
func execString(name string, arg ...string) string {
	//fmt.Println(name, arg)
	cmd := exec.Command(name, arg...)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		//fmt.Println(fmt.Sprint(err) + ": " + string(stdout))
		fmt.Println("CMD ERROR")
		return string(stdout)
	}
	return string(stdout)
}
