// Copyright (c) 2026 Vladislav Plotkin
// SPDX-License-Identifier: MIT
package aura

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// StateFilePath — путь к файлу состояния
const StateFilePath = "/tmp/aura_led2_state.json"

// LCDLedCount — количество LED для LCD (LED1..LED32)
const LCDLedCount = 32

// State хранит объединённое состояние для канала led2:
// Indicator — цвет LED0 (индикатор)
// LCD — цвета LED1..LED32 (символы LCD)
type State struct {
	Indicator [3]byte     `json:"indicator"`
	LCD       [32][3]byte `json:"lcd"`
}

// defaultState возвращает состояние по умолчанию (все выключено)
func defaultState() *State {
	return &State{}
}

// LoadState загружает состояние из JSON-файла.
// Если файл отсутствует или повреждён — возвращает состояние по умолчанию.
func LoadState() *State {
	data, err := os.ReadFile(StateFilePath)
	if err != nil {
		return defaultState()
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return defaultState()
	}
	return &s
}

// SaveState атомарно сохраняет состояние в JSON-файл
// (запись во временный файл + os.Rename).
func SaveState(s *State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(StateFilePath)
	tmp, err := os.CreateTemp(dir, ".aura_led2_state_")
	if err != nil {
		return err
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()

	return os.Rename(tmp.Name(), StateFilePath)
}

// GetFullColors собирает полный 33-LED буфер из состояния:
// colors[0] = indicator, colors[1..32] = LCD
func GetFullColors(s *State) [][3]byte {
	full := make([][3]byte, 0, 1+LCDLedCount)
	full = append(full, s.Indicator)
	full = append(full, s.LCD[:]...)
	return full
}

// SetIndicator устанавливает цвет индикатора в состоянии
func SetIndicator(s *State, r, g, b byte) {
	s.Indicator = [3]byte{r, g, b}
}

// SetLCDColors устанавливает цвета LCD в состоянии (ровно 32 записи)
func SetLCDColors(s *State, colors [][3]byte) {
	for i := 0; i < LCDLedCount && i < len(colors); i++ {
		s.LCD[i] = colors[i]
	}
	for i := len(colors); i < LCDLedCount; i++ {
		s.LCD[i] = [3]byte{0, 0, 0}
	}
}
