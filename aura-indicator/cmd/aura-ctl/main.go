// Copyright (c) 2026 Vladislav Plotkin
// SPDX-License-Identifier: MIT
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"aura-indicator/pkg/aura"
)

var devicePath string

func main() {
	flag.StringVar(&devicePath, "d", "", "HID device path (auto-detect if empty)")
	flag.StringVar(&devicePath, "device", "", "HID device path (auto-detect if empty)")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Usage: aura-ctl [-d device] <command> [args...]\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  detect                          List connected AURA devices\n")
		fmt.Fprintf(os.Stderr, "  status                          Show device status\n")
		fmt.Fprintf(os.Stderr, "  set <channel> <mode> [color]    Set effect mode\n")
		fmt.Fprintf(os.Stderr, "  direct <channel> <color>...     Set direct colors\n")
		fmt.Fprintf(os.Stderr, "  off <channel>                   Turn off channel\n")
		fmt.Fprintf(os.Stderr, "  lcd <text>                      Write text to LCD (full screen)\n")
		fmt.Fprintf(os.Stderr, "  lcd-line <row> <text>           Write one line to LCD\n")
		fmt.Fprintf(os.Stderr, "  blink <r,g,b> [times] [ms]      Blink indicator\n")
		fmt.Fprintf(os.Stderr, "  author                          Show author info\n")
		os.Exit(1)
	}

	cmd := flag.Arg(0)
	args := flag.Args()[1:]

	switch cmd {
	case "detect":
		cmdDetect()
	case "status":
		cmdStatus()
	case "set":
		cmdSet(args)
	case "direct":
		cmdDirect(args)
	case "off":
		cmdOff(args)
	case "lcd":
		cmdLCD(args)
	case "lcd-line":
		cmdLCDLine(args)
	case "blink":
		cmdBlink(args)
	case "author":
		cmdAuthor()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

func cmdDetect() {
	devices, err := aura.DiscoverDevices()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(devices) == 0 {
		fmt.Println("No ASUS AURA devices detected.")
		return
	}
	for _, d := range devices {
		fmt.Printf("Device: %s\n", d.Path)
		fmt.Printf("  Vendor:  0x%04X\n", d.Vendor)
		fmt.Printf("  Product: 0x%04X\n", d.Product)
		if d.Name != "" {
			fmt.Printf("  Name:    %s\n", d.Name)
		}
	}
}

func cmdStatus() {
	dev, err := aura.OpenDevice(devicePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer dev.Close()

	fmt.Printf("Device:      %s\n", dev.Path())
	fmt.Printf("Firmware:    %s\n", dev.Firmware())
	fmt.Printf("Fixed LEDs:  %d\n", dev.NumTotal())
	fmt.Printf("Addr LEDs:   %d\n", dev.NumAddr())
	fmt.Printf("Channels:\n")
	for _, ch := range dev.Channels() {
		typ := "ARGB"
		if !ch.IsAddr {
			typ = "RGB"
		}
		fmt.Printf("  %s: effect=0x%02X direct=0x%02X start=%d n=%d [%s]\n",
			ch.Name, ch.EffectCh, ch.DirectCh, ch.StartLED, ch.NumLEDs, typ)
	}
}

func cmdSet(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: aura-ctl set <channel> <mode> [color]\n")
		os.Exit(1)
	}

	channel := args[0]
	mode := args[1]
	var color *[3]byte

	if len(args) >= 3 {
		r, g, b, err := parseColor(args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid color: %v\n", err)
			os.Exit(1)
		}
		c := [3]byte{r, g, b}
		color = &c
	}

	dev, err := aura.OpenDevice(devicePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer dev.Close()

	if err := dev.SetMode(channel, mode, color); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdDirect(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: aura-ctl direct <channel> <color>...\n")
		os.Exit(1)
	}

	channel := args[0]
	colors := make([][3]byte, 0, len(args)-1)

	for i := 1; i < len(args); i++ {
		r, g, b, err := parseColor(args[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid color %s: %v\n", args[i], err)
			os.Exit(1)
		}
		colors = append(colors, [3]byte{r, g, b})
	}

	dev, err := aura.OpenDevice(devicePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer dev.Close()

	if err := dev.SetDirect(channel, colors); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdOff(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: aura-ctl off <channel>\n")
		os.Exit(1)
	}

	channel := args[0]

	dev, err := aura.OpenDevice(devicePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer dev.Close()

	if err := dev.Off(channel); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdLCD(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: aura-ctl lcd <text>\n")
		os.Exit(1)
	}

	text := strings.Join(args, " ")

	_, err := aura.SendLCDText(text, -1, devicePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdLCDLine(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: aura-ctl lcd-line <row> <text>\n")
		os.Exit(1)
	}

	row, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid row: %v\n", err)
		os.Exit(1)
	}

	text := strings.Join(args[1:], " ")

	_, err = aura.SendLCDText(text, row, devicePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdBlink(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: aura-ctl blink <r,g,b> [times] [interval_ms]\n")
		os.Exit(1)
	}

	r, g, b, err := parseColor(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid color: %v\n", err)
		os.Exit(1)
	}

	times := 3
	if len(args) > 1 {
		times, err = strconv.Atoi(args[1])
		if err != nil || times < 1 {
			times = 3
		}
	}

	interval := 300
	if len(args) > 2 {
		interval, err = strconv.Atoi(args[2])
		if err != nil || interval < 50 {
			interval = 300
		}
	}

	for i := 0; i < times; i++ {
		aura.SetIndicatorDirect(r, g, b, devicePath)
		time.Sleep(time.Duration(interval) * time.Millisecond)
		aura.SetIndicatorDirect(0, 0, 0, devicePath)
		if i < times-1 {
			time.Sleep(time.Duration(interval) * time.Millisecond)
		}
	}
}

func cmdAuthor() {
	fmt.Println("AURA LED Controller Toolchain")
	fmt.Println("Author: Vladislav Plotkin")
	fmt.Println("License: MIT")
	fmt.Println("Repository: github.com/vladislav-plotkin/aura")

	// Show on LCD: row 0 = "Vladislav       ", row 1 = "Plotkin         "
	aura.SendLCDText("Vladislav       Plotkin         ", -1, "")
}

func parseColor(s string) (byte, byte, byte, error) {
	parts := strings.SplitN(s, ",", 3)
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("expected R,G,B got %q", s)
	}
	r, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid R: %w", err)
	}
	g, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid G: %w", err)
	}
	b, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid B: %w", err)
	}
	if r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 {
		return 0, 0, 0, fmt.Errorf("values out of range (0-255): %s", s)
	}
	return byte(r), byte(g), byte(b), nil
}
