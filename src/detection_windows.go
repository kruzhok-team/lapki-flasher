//go:build windows
// +build windows

package main

import (
	"fmt"
	"log"

	"github.com/google/gousb"
	"golang.org/x/sys/windows/registry"
)

func findPortName(desc *gousb.DeviceDesc) string {
	cmdPathPattern := fmt.Sprintf("USB\\VID_%s&PID_%s*", desc.Vendor.String(), desc.Product.String())
	cmdPattern := fmt.Sprintf("Get-PnpDevice -status 'ok' -InstanceId '%s' | Select-Object -Property InstanceId", cmdPathPattern)
	//fmt.Println(cmdPattern)
	cmdResult := execString("powershell", cmdPattern)
	var possiblePathes []string
	curStr := ""
	for _, v := range cmdResult {
		if v == '\n' {
			if len(curStr) > 0 && curStr[0] == 'U' {
				possiblePathes = append(possiblePathes, curStr)
			}
			curStr = ""
			continue
		}
		if v == 13 {
			continue
		}
		curStr += string(v)
	}
	for _, path := range possiblePathes {
		keyPath := fmt.Sprintf("SYSTEM\\CurrentControlSet\\Enum\\%s\\Device Parameters", path)
		key, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
		if err != nil {
			log.Fatal("Registry error:", err)
		}
		s, _, err := key.GetStringValue("PortName")
		fmt.Println("PORT NAME", s)
		if err == registry.ErrNotExist {
			fmt.Println("not exists")
			continue
		}
		if err != nil {
			log.Fatal("Get port name error:", err.Error())
		}
		return s
	}
	return NOT_FOUND
}
