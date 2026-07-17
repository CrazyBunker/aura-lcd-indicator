// Copyright (c) 2026 Vladislav Plotkin
// SPDX-License-Identifier: MIT
//go:build !linux

package aura

import "fmt"

// DeviceInfo содержит информацию о найденном устройстве
type DeviceInfo struct {
	Path    string
	Vendor  int
	Product int
	Name    string
}

// DiscoverDevices не поддерживается на этой платформе
func DiscoverDevices() ([]DeviceInfo, error) {
	return nil, fmt.Errorf("aura: HID device discovery is not supported on this platform")
}

// autoDetectDevice не поддерживается на этой платформе
func autoDetectDevice() (string, error) {
	return "", fmt.Errorf("aura: HID device discovery is not supported on this platform")
}

// CheckPermissions не поддерживается на этой платформе
func CheckPermissions(path string) bool {
	return false
}

// ReadRaw не поддерживается на этой платформе
func (d *AuraDevice) ReadRaw() ([]byte, error) {
	return nil, fmt.Errorf("aura: raw read not supported on this platform")
}
