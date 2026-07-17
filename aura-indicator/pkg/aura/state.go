// Copyright (c) 2026 Vladislav Plotkin
// SPDX-License-Identifier: MIT
package aura

import (
	"encoding/json"
	"os"
	"path/filepath"
)

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

// stateFilePath возвращает путь к файлу состояния — рядом с бинарником.
func stateFilePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "aura_led2_state.json"
	}
	dir := filepath.Dir(exe)
	return filepath.Join(dir, "aura_led2_state.json")
}

// LoadState загружает состояние из файла рядом с бинарником.
// Если файл не найден или повреждён — возвращает состояние по умолчанию.
func LoadState() *State {
	data, err := os.ReadFile(stateFilePath())
	if err != nil {
		return defaultState()
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return defaultState()
	}
	return &s
}

// SaveState сохраняет состояние в файл рядом с бинарником.
func SaveState(s *State) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFilePath(), data, 0644)
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
