// Copyright (c) 2026 Vladislav Plotkin
// SPDX-License-Identifier: MIT
package aura

import (
	"fmt"
	"strings"
)

// Константы USB HID устройства AURA
const (
	VID_ASUS  = 0x0B05
	REPORT_LEN = 65
	CMD_PREFIX = 0xEC
)

// PIDS_AURA — известные Product ID AURA LED контроллеров
var PIDS_AURA = map[uint16]bool{
	0x18F3: true,
	0x19AF: true,
	0x1939: true,
	0x18F5: true, // AURA LED Controller 2
	0x1854: true,
	0x1866: true,
}

// MODE_MAP — имена режимов → байт-код
var MODE_MAP = map[string]byte{
	"off":                        0x00,
	"static":                     0x01,
	"breathing":                  0x02,
	"flashing":                   0x03,
	"spectrum_cycle":             0x04,
	"rainbow":                    0x05,
	"spectrum_cycle_breathing":   0x06,
	"chase_fade":                0x07,
	"spectrum_cycle_chase_fade":  0x08,
	"chase":                      0x09,
	"spectrum_cycle_chase":       0x0A,
	"spectrum_cycle_wave":        0x0B,
	"chase_rainbow_pulse":        0x0C,
	"rainbow_flicker":            0x0D,
	"gentle_transition":          0x10,
	"wave_propagation":           0x11,
	"wave_propagation_pause":     0x12,
	"red_pulse":                  0x13,
}

// MODE_NAMES — обратный маппинг: байт-код → имя
var MODE_NAMES = func() map[byte]string {
	m := make(map[byte]string, len(MODE_MAP))
	for k, v := range MODE_MAP {
		m[v] = k
	}
	return m
}()

// BuildSetMode формирует пакет установки эффекта.
// channelType: 0x00 — RGB (fixed), 0x01 — ARGB (addressable)
func BuildSetMode(channelType byte, mode byte) [65]byte {
	return [65]byte{
		CMD_PREFIX, 0x35, channelType, 0x00, 0x00, mode,
	}
}

// BuildSetColor формирует пакет установки цвета для канала.
// channelID — номер канала (0-based).
func BuildSetColor(channelID byte, r, g, b byte) [65]byte {
	return [65]byte{
		CMD_PREFIX, 0x36, channelID, 0x00, r, g, b,
	}
}

// BuildCommit формирует пакет фиксации (commit) настроек.
func BuildCommit() [65]byte {
	return [65]byte{
		CMD_PREFIX, 0x3F, 0x55,
	}
}

// BuildEndEffect формирует пакет завершения эффекта (вход в эффект-режим).
func BuildEndEffect() [65]byte {
	return [65]byte{
		CMD_PREFIX, 0x35, 0x00, 0x00, 0x00, 0xFF,
	}
}

// BuildEndDirect формирует пакет завершения direct-режима (выход из него).
func BuildEndDirect() [65]byte {
	return [65]byte{
		CMD_PREFIX, 0x35, 0x00, 0x00, 0x00, 0x00,
	}
}

// BuildDirectPackets формирует пачку пакетов для direct-режима.
// channelDirectID — direct channel index.
// colors — срез цветов [][3]byte{R, G, B}.
// startLED — начальный LED (обычно 0).
// Каждый пакет содержит максимум MaxLEDsPerPacket=20 LED.
// Последний пакет в серии получает apply-флаг 0x80.
func BuildDirectPackets(channelDirectID byte, colors [][3]byte, startLED int) [][65]byte {
	const maxLEDsPerPacket = 20

	if len(colors) == 0 {
		return nil
	}

	var packets [][65]byte
	totalSent := 0

	for totalSent < len(colors) {
		remain := len(colors) - totalSent
		count := remain
		if count > maxLEDsPerPacket {
			count = maxLEDsPerPacket
		}

		chID := channelDirectID
		if totalSent+count >= len(colors) {
			chID |= 0x80 // apply flag on last packet
		}

		var pkt [65]byte
		pkt[0] = CMD_PREFIX
		pkt[1] = 0x40
		pkt[2] = chID
		pkt[3] = byte(startLED + totalSent)
		pkt[4] = byte(count)

		idx := 5
		for j := 0; j < count; j++ {
			c := colors[totalSent+j]
			pkt[idx] = c[0] // R
			pkt[idx+1] = c[1] // G
			pkt[idx+2] = c[2] // B
			idx += 3
		}

		totalSent += count
		packets = append(packets, pkt)
	}

	return packets
}

// BuildReadFirmware формирует пакет чтения версии прошивки.
func BuildReadFirmware() [65]byte {
	return [65]byte{
		CMD_PREFIX, 0x82,
	}
}

// BuildReadConfig формирует пакет чтения конфигурации.
func BuildReadConfig() [65]byte {
	return [65]byte{
		CMD_PREFIX, 0xB0,
	}
}

// BuildMainboardEffect формирует пакет эффекта для материнской платы (OpenRGB).
func BuildMainboardEffect(effectCh byte, mode byte, shutdown byte) [65]byte {
	return [65]byte{
		CMD_PREFIX, 0x35, effectCh, 0x00, shutdown, mode,
	}
}

// BuildMainboardColor формирует пакет цвета для материнской платы.
// mask — битовая маска LED (16 бит).
// ledArea — 60 байт цвета.
// shutdown — флаг выключения.
func BuildMainboardColor(mask uint16, ledArea [60]byte, shutdown byte) [65]byte {
	var pkt [65]byte
	pkt[0] = CMD_PREFIX
	pkt[1] = 0x36
	pkt[2] = byte(mask >> 8)
	pkt[3] = byte(mask & 0xFF)
	pkt[4] = shutdown
	copy(pkt[5:], ledArea[:])
	return pkt
}

// BuildDirectEnter формирует пакет входа в direct-режим для канала.
func BuildDirectEnter(effectCh byte) [65]byte {
	return [65]byte{
		CMD_PREFIX, 0x35, effectCh, 0x00, 0x00, 0xFF,
	}
}

// HexDump форматирует байты в hex-строку, обрезая до maxLen байт.
func HexDump(data []byte, maxLen int) string {
	if len(data) == 0 {
		return "(empty)"
	}
	show := data
	if len(show) > maxLen {
		show = show[:maxLen]
	}
	parts := make([]string, len(show))
	for i, b := range show {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	s := strings.Join(parts, " ")
	if len(data) > maxLen {
		s += fmt.Sprintf(" ... (%d more bytes)", len(data)-maxLen)
	}
	return s
}
