package main

import (
	_ "embed"
	"encoding/json"
)

//go:embed device_list.JSON
var mainBoardTemplatesRaw []byte

// шаблон описания устройства с ID и BootloaderID
type BoardTemplate struct {
	ID           int
	VendorIDs    []string
	ProductIDs   []string
	Name         string
	Controller   string
	Programmer   string
	BootloaderID int
}

// шаблон описания устройства, хранящегося в json файле, без ID и BootloaderID
type BoardTemplateRaw struct {
	VendorIDs     []string `json:"vendorIDs"`
	ProductIDs    []string `json:"productIDs"`
	Name          string   `json:"name"`
	Controller    string   `json:"controller"`
	Programmer    string   `json:"programmer"`
	HasBootloader bool     `json:"hasBootloader"`
}

// загрузка шаблонов устройств и генерация ID для них
func loadTemplatesFromRaw(boardTemplatesRaw []byte) []BoardTemplate {
	var rawTamplates []BoardTemplateRaw
	json.Unmarshal(boardTemplatesRaw, &rawTamplates)
	tempNum := len(rawTamplates)
	var boardTemplatesID = make([]BoardTemplate, tempNum)
	for ID, temp := range rawTamplates {
		boardTemplatesID[ID] = BoardTemplate{
			ID:         ID,
			VendorIDs:  temp.VendorIDs,
			ProductIDs: temp.ProductIDs,
			Name:       temp.Name,
			Controller: temp.Controller,
			Programmer: temp.Programmer,
		}
		if temp.HasBootloader {
			boardTemplatesID[ID].BootloaderID = ID + 1
		} else {
			boardTemplatesID[ID].BootloaderID = -1
		}
	}
	return boardTemplatesID
}

// находит шаблон платы по его id
func findTemplateByID(boardID int) *BoardTemplate {
	var template BoardTemplate
	if boardID < len(detector.boardTemplates) {
		template = detector.boardTemplates[boardID]
		// ожидается, что в файле с шаблонами прошивок (device_list.JSON) нумеровка индексов будет идти по порядку, но если это не так, то придётся перебать все шаблоны
		if template.ID != boardID {
			foundCorrectBootloader := false
			for _, templ := range detector.boardTemplates {
				if templ.ID == boardID {
					template = templ
					foundCorrectBootloader = true
					break
				}
			}
			if foundCorrectBootloader {
				printLog("Не найден шаблон для устройства")
				return nil
			}
		}
	}
	return &template
}
