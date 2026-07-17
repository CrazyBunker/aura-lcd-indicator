// Copyright (c) 2026 Vladislav Plotkin
// SPDX-License-Identifier: MIT
//go:build linux

package aura

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DeviceInfo содержит информацию о найденном устройстве
type DeviceInfo struct {
	Path    string
	Vendor  int
	Product int
	Name    string
}

// DiscoverDevices сканирует hidraw устройства и возвращает список AURA-совместимых
func DiscoverDevices() ([]DeviceInfo, error) {
	hidrawDir := "/sys/class/hidraw"
	entries, err := os.ReadDir(hidrawDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", hidrawDir, err)
	}

	var devices []DeviceInfo

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "hidraw") {
			continue
		}

		path := filepath.Join(hidrawDir, entry.Name())

		vendor, product, name := readDeviceInfo(path)
		if vendor == VID_ASUS && PIDS_AURA[uint16(product)] {
			devPath := "/dev/" + entry.Name()
			devices = append(devices, DeviceInfo{
				Path:    devPath,
				Vendor:  vendor,
				Product: product,
				Name:    name,
			})
		}
	}

	return devices, nil
}

// autoDetectDevice находит первое AURA-устройство
func autoDetectDevice() (string, error) {
	devices, err := DiscoverDevices()
	if err != nil {
		return "", err
	}
	if len(devices) == 0 {
		return "", fmt.Errorf("no ASUS AURA device found")
	}
	return devices[0].Path, nil
}

// CheckPermissions проверяет доступ к устройству
func CheckPermissions(path string) bool {
	f, err := os.OpenFile(path, os.O_RDWR, 0666)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// readDeviceInfo читает vendor, product, name из sysfs uevent
func readDeviceInfo(sysfsPath string) (int, int, string) {
	ueventPath := filepath.Join(sysfsPath, "device", "uevent")
	data, err := os.ReadFile(ueventPath)
	if err != nil {
		// Пробуем альтернативный путь
		ueventPath = filepath.Join(sysfsPath, "device", "modalias")
		data2, err2 := os.ReadFile(ueventPath)
		if err2 != nil {
			return 0, 0, ""
		}
		// Парсим modalias: usb:v0B05p18F3...
		s := string(data2)
		var v, p int
		if _, err := fmt.Sscanf(s, "usb:v%04Xp%04X", &v, &p); err == nil {
			return v, p, ""
		}
		return 0, 0, ""
	}

	vendor := 0
	product := 0
	name := ""

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "HID_ID=") {
			// HID_ID=0001:00000B05:000018F3
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				fields := strings.Split(parts[1], ":")
				if len(fields) >= 3 {
					v, _ := strconv.ParseInt(fields[1], 16, 64)
					p, _ := strconv.ParseInt(fields[2], 16, 64)
					vendor = int(v)
					product = int(p)
				}
			}
		}
		if strings.HasPrefix(line, "HID_NAME=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name = parts[1]
			}
		}
		if strings.HasPrefix(line, "HID_UNIQ=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && name == "" {
				name = parts[1]
			}
		}
	}

	return vendor, product, name
}

// ReadRaw — простой гарантированный read с устройства
// Для hidraw на Linux read всегда блокирующий, возвращает 1 репорт.
func (d *AuraDevice) ReadRaw() ([]byte, error) {
	d.mu.Lock()
	fd := d.fd
	d.mu.Unlock()

	if fd == nil {
		return nil, fmt.Errorf("aura: device not open")
	}

	buf := make([]byte, REPORT_LEN)
	n, err := fd.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("aura: read error: %w", err)
	}
	return buf[:n], nil
}
