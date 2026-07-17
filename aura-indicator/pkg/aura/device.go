// Copyright (c) 2026 Vladislav Plotkin
// SPDX-License-Identifier: MIT
package aura

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// ----- ChannelInfo -----
type ChannelInfo struct {
	Name     string // "led1", "led2", ...
	EffectCh byte   // channel index for effect commands
	DirectCh byte   // channel index for direct mode
	StartLED int    // first LED index
	NumLEDs  int    // number of LEDs
	IsAddr   bool   // true = ARGB (addressable), false = fixed RGB
}

// ----- AuraDevice -----
type AuraDevice struct {
	path     string
	fd       *os.File
	firmware string
	config   []byte
	numTotal int        // fixed (RGB) LEDs count
	numAddr  int        // addressable (ARGB) LEDs count
	channels []ChannelInfo
	verbose  bool
	mu       sync.Mutex
}

// OpenDevice открывает устройство AURA. Если devicePath пустой — автоопределение.
func OpenDevice(devicePath string) (*AuraDevice, error) {
	path := devicePath
	if path == "" {
		var err error
		path, err = autoDetectDevice()
		if err != nil {
			return nil, fmt.Errorf("aura: device not found: %w", err)
		}
	}

	dev := &AuraDevice{path: path}
	if err := dev.Open(); err != nil {
		return nil, err
	}
	return dev, nil
}

// Open открывает hidraw, читает версию и конфиг.
func (d *AuraDevice) Open() error {
	var err error
	d.fd, err = os.OpenFile(d.path, os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("aura: cannot open %s: %w", d.path, err)
	}

	// Читаем версию (необязательно, может не быть на старых прошивках)
	if ver, err := d.GetVersion(); err == nil {
		d.firmware = ver
	}

	// Читаем конфигурацию каналов
	argbCount, rgbCount, err := d.GetConfig()
	if err != nil {
		// даже если конфиг не прочитался, можем работать с дефолтами
		d.numAddr = 1  // хотя бы led2
		d.numTotal = 0
	} else {
		d.numAddr = argbCount
		d.numTotal = rgbCount
	}
	d.buildChannels()
	return nil
}

// Close закрывает устройство.
func (d *AuraDevice) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.fd != nil {
		d.fd.Close()
		d.fd = nil
	}
}

// buildChannels формирует список каналов на основе прочитанного конфига.
func (d *AuraDevice) buildChannels() {
	d.channels = nil

	// led1 — фиксированный RGB (если есть)
	if d.numTotal > 0 {
		d.channels = append(d.channels, ChannelInfo{
			Name:     "led1",
			EffectCh: 0,
			DirectCh: 0x04,
			StartLED: 0,
			NumLEDs:  d.numTotal,
			IsAddr:   false,
		})
	}

	// led2, led3, ... — addressable
	// Внимание: num_leds=1 для addressable (как в OpenRGB и Python-версии),
	// чтобы эффект-режим на led2 не затирал всю LCD-ленту (33 LED).
	// Для direct-режима количество LED не ограничено этим полем.
	for i := 0; i < d.numAddr; i++ {
		name := fmt.Sprintf("led%d", i+2)
		startLED := i
		if d.numTotal > 0 {
			startLED = d.numTotal + i
		}
		d.channels = append(d.channels, ChannelInfo{
			Name:     name,
			EffectCh: byte(i),
			DirectCh: byte(i),       // 0, 1, 2 для ARGB-каналов
			StartLED: startLED,
			NumLEDs:  1,             // эффект-режим управляет только 1 LED
			IsAddr:   true,
		})
	}

	// Если конфиг не прочитался, создаём дефолтный led2
	if len(d.channels) == 0 {
		d.channels = append(d.channels, ChannelInfo{
			Name:     "led2",
			EffectCh: 0,
			DirectCh: 0,
			StartLED: 0,
			NumLEDs:  1,
			IsAddr:   true,
		})
	}
}

// SendRaw отправляет 65-байтный HID-репорт.
func (d *AuraDevice) SendRaw(packet [65]byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.fd == nil {
		return fmt.Errorf("aura: device not open")
	}

	n, err := d.fd.Write(packet[:])
	if err != nil {
		return fmt.Errorf("aura: write error: %w", err)
	}
	if n != REPORT_LEN {
		return fmt.Errorf("aura: wrote %d bytes, expected %d", n, REPORT_LEN)
	}
	return nil
}

// ReadResponse читает ответ от устройства с таймаутом.
func (d *AuraDevice) ReadResponse(timeoutMs int) ([]byte, error) {
	d.mu.Lock()
	fd := d.fd
	d.mu.Unlock()

	if fd == nil {
		return nil, fmt.Errorf("aura: device not open")
	}

	type result struct {
		data []byte
		err  error
	}

	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, REPORT_LEN)
		n, err := fd.Read(buf)
		if err != nil {
			ch <- result{nil, err}
		} else {
			ch <- result{buf[:n], nil}
		}
	}()

	if timeoutMs <= 0 {
		timeoutMs = 1000
	}

	timer := time.NewTimer(time.Duration(timeoutMs) * time.Millisecond)
	defer timer.Stop()

	select {
	case r := <-ch:
		return r.data, r.err
	case <-timer.C:
		return nil, fmt.Errorf("aura: read timeout (%dms)", timeoutMs)
	}
}

// GetVersion читает версию прошивки.
func (d *AuraDevice) GetVersion() (string, error) {
	pkt := BuildReadFirmware()
	if err := d.SendRaw(pkt); err != nil {
		return "", err
	}
	resp, err := d.ReadResponse(500)
	if err != nil {
		return "", err
	}
	if len(resp) < 6 {
		return "", fmt.Errorf("short firmware response")
	}
	// Версия в байтах 2..5 как строковые символы
	ver := strings.TrimRight(string(resp[2:6]), "\x00")
	return ver, nil
}

// GetConfig читает конфигурацию устройства.
// Возвращает (argbCount, rgbCount, error).
func (d *AuraDevice) GetConfig() (int, int, error) {
	pkt := BuildReadConfig()
	if err := d.SendRaw(pkt); err != nil {
		return 0, 0, err
	}
	resp, err := d.ReadResponse(500)
	if err != nil {
		return 0, 0, err
	}
	d.config = resp

	// Парсим конфиг (OpenRGB convention)
	// config[0x02] = number of addressable LEDs / channels
	// config[0x1B] = number of fixed (RGB) LEDs
	var argbCount int
	var rgbCount int

	if len(resp) > 0x02 {
		argbCount = int(resp[0x02])
	}
	if len(resp) > 0x1B {
		rgbCount = int(resp[0x1B])
	}
	// Падение: некоторые прошивки возвращают 0xFF для ARGB
	if argbCount > 10 || argbCount == 0 {
		argbCount = 1
	}

	return argbCount, rgbCount, nil
}

// getChannelByName ищет канал по имени (led1, led2, sync).
func (d *AuraDevice) getChannelByName(name string) ([]ChannelInfo, error) {
	name = strings.ToLower(name)

	if name == "sync" {
		return d.channels, nil
	}

	for _, ch := range d.channels {
		if ch.Name == name {
			return []ChannelInfo{ch}, nil
		}
	}
	return nil, fmt.Errorf("aura: unknown channel %q (available: %s)", name, d.channelNames())
}

func (d *AuraDevice) channelNames() string {
	names := make([]string, len(d.channels))
	for i, ch := range d.channels {
		names[i] = ch.Name
	}
	return strings.Join(names, ", ")
}

// SetMode устанавливает эффект для канала.
// channel: "led1", "led2", или "sync" (все каналы).
// mode: имя режима из MODE_MAP.
// color: опциональный цвет для static-режима (может быть nil).
//
// Для led2 используется direct-режим с мержем из state-файла,
// чтобы не затирать LCD-текст (LED1..LED32).
func (d *AuraDevice) SetMode(channel, mode string, color *[3]byte) error {
	// Для led2 — через state + direct (индикатор не затирает LCD)
	if channel == "led2" {
		state := LoadState()
		if color != nil {
			SetIndicator(state, color[0], color[1], color[2])
		} else {
			SetIndicator(state, 0, 0, 0)
		}
		if err := SaveState(state); err != nil {
			return err
		}
		return d.SetDirect("led2", GetFullColors(state))
	}

	channels, err := d.getChannelByName(channel)
	if err != nil {
		return err
	}

	modeByte, ok := MODE_MAP[mode]
	if !ok {
		return fmt.Errorf("aura: unknown mode %q (available: %v)", mode, mapKeys(MODE_MAP))
	}

	for _, ch := range channels {
		channelType := byte(0x00)
		if ch.IsAddr {
			channelType = 0x01
		}

		// Effect packet
		pkt := BuildSetMode(channelType, modeByte)
		if err := d.SendRaw(pkt); err != nil {
			return fmt.Errorf("aura: effect packet failed for %s: %w", ch.Name, err)
		}

		// Color packet (если нужен)
		if color != nil {
			cpkt := BuildSetColor(ch.EffectCh, color[0], color[1], color[2])
			if err := d.SendRaw(cpkt); err != nil {
				return fmt.Errorf("aura: color packet failed for %s: %w", ch.Name, err)
			}
		}

		// Commit
		if err := d.SendRaw(BuildCommit()); err != nil {
			return fmt.Errorf("aura: commit failed for %s: %w", ch.Name, err)
		}
	}

	return nil
}

// SetDirect устанавливает direct-цвета для канала.
// Для led2 загружает мерж с индикатором из state-файла.
func (d *AuraDevice) SetDirect(channel string, colors [][3]byte) error {
	if len(colors) == 0 {
		return fmt.Errorf("aura: no colors provided for %s", channel)
	}

	channels, err := d.getChannelByName(channel)
	if err != nil {
		return err
	}

	for _, ch := range channels {
		// Входим в direct mode
		if err := d.SendRaw(BuildDirectEnter(ch.EffectCh)); err != nil {
			return fmt.Errorf("aura: direct enter failed for %s: %w", ch.Name, err)
		}

		packets := BuildDirectPackets(ch.DirectCh, colors, 0)
		for i, pkt := range packets {
			if err := d.SendRaw(pkt); err != nil {
				return fmt.Errorf("aura: direct packet %d failed for %s: %w", i, ch.Name, err)
			}
		}

		// Не отправляем EndDirect — оставляем в direct-режиме
		// (как делает OpenRGB)
	}

	return nil
}

// Off выключает канал.
// Для "sync" — все каналы.
// Для led2 — только индикатор (LED0), LCD-текст сохраняется.
func (d *AuraDevice) Off(channel string) error {
	if channel == "led2" {
		state := LoadState()
		SetIndicator(state, 0, 0, 0)
		if err := SaveState(state); err != nil {
			return err
		}
		return d.SetDirect("led2", GetFullColors(state))
	}
	return d.SetMode(channel, "off", &[3]byte{0, 0, 0})
}

// SyncOff выключает все каналы.
func (d *AuraDevice) SyncOff() error {
	return d.Off("sync")
}

// ----- helpers -----

func mapKeys(m map[string]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Геттеры для AuraDevice

func (d *AuraDevice) Path() string      { return d.path }
func (d *AuraDevice) Firmware() string  { return d.firmware }
func (d *AuraDevice) NumTotal() int     { return d.numTotal }
func (d *AuraDevice) NumAddr() int      { return d.numAddr }
func (d *AuraDevice) Channels() []ChannelInfo {
	cp := make([]ChannelInfo, len(d.channels))
	copy(cp, d.channels)
	return cp
}
