// Copyright (c) 2026 Vladislav Plotkin
// SPDX-License-Identifier: MIT
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"aura-indicator/pkg/aura"
)

// toolCall represents a fixed-state notification tool with its RGB colour.
type toolCall struct {
	r, g, b uint8
}

var tools = map[string]toolCall{
	"aura_notify_start":    {0, 0, 255}, // blue
	"aura_notify_done":     {0, 255, 0}, // green
	"aura_notify_off":      {0, 0, 0},   // off
	"aura_notify_question": {255, 0, 0}, // red
}

// ── blink state ──────────────────────────────────────────────────────────────

var (
	blinkStop chan struct{}
	blinkMu   sync.Mutex
)

// stopBlink kills any running background blink goroutine.
func stopBlink() {
	blinkMu.Lock()
	defer blinkMu.Unlock()
	if blinkStop != nil {
		close(blinkStop)
		blinkStop = nil
	}
}

// startBlink runs a blink loop in a background goroutine.
func startBlink(r, g, b byte, times, intervalMs int, devicePath string) {
	blinkMu.Lock()
	if blinkStop != nil {
		close(blinkStop)
	}
	ch := make(chan struct{})
	blinkStop = ch
	blinkMu.Unlock()

	go func() {
		for i := 0; i < times; i++ {
			_ = aura.SetIndicatorDirect(r, g, b, devicePath)
			select {
			case <-ch:
				return
			case <-time.After(time.Duration(intervalMs) * time.Millisecond):
			}
			_ = aura.SetIndicatorDirect(0, 0, 0, devicePath)
			if i < times-1 {
				select {
				case <-ch:
					return
				case <-time.After(time.Duration(intervalMs) * time.Millisecond):
				}
			}
		}
	}()
}

// ── MCP JSON-RPC types ──────────────────────────────────────────────────────

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type toolCallResult struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"isError"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type resourceDef struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// ── resources ──────────────────────────────────────────────────────────────

const (
	resLcdURI           = "aura-indicator://lcd"
	resNotificationsURI = "aura-indicator://notifications"
	resAuthorURI        = "aura-indicator://author"
)

const resLcdText = `# Display — 16x2 Character Screen

A 2-line character display, 16 characters per line (32 total when both lines used).

## Character table
Only ASCII is supported: Latin letters (A-Z, a-z), digits (0-9), and basic
punctuation/space. Anything else (Cyrillic, emoji, non-printable) is replaced
with a space by the display firmware. Keep text in English/ASCII.

## Limits and behavior
| Mode | Max chars | Parameter | On overflow |
|------|-----------|-----------|-------------|
| Full screen (both lines) | 32 | row omitted | Truncated, warning returned — rephrase to fit 32 |
| Single line (row 0 or 1) | 16 | row = 0 or 1 | Truncated, warning returned — rephrase to fit 16 |

## What to print
The screen is small — print SHORT status/debug info only, in English/ASCII:
- Current action keyword: "thinking", "planning", "coding", "reviewing",
  "testing", "building docs", "deploying"
- Progress markers: "step 3/5", "waiting input", "done", "error"
- Short task keywords or stage names
- Anything that helps the user glance at what the agent is doing

Do NOT print: long sentences, Russian/Cyrillic, emoji, or full error dumps.
Keep each line <= 16 chars; full screen <= 32 chars.

## MCP examples
Full screen:
  {"name":"aura_lcd_print","arguments":{"text":"Building plan..."}}
Line 0:
  {"name":"aura_lcd_print","arguments":{"text":"thinking","row":0}}
Line 1:
  {"name":"aura_lcd_print","arguments":{"text":"step 3/5","row":1}}
`

const resNotificationsText = `# Notifications — when to call

The agent MUST use notifications to keep the user informed. Calling them is
mandatory in the situations below.

| Tool | When to call (MANDATORY) |
|------|--------------------------|
| aura_notify_start | As soon as a new task/pipeline begins. |
| aura_notify_done | When the whole pipeline is finished AND verified. |
| aura_notify_off | (1) When a NEW task/instruction arrives. (2) As soon as a REPLY from the user is received — clear the notification. |
| aura_notify_question | Whenever the agent asks the user something and WAITS for an answer (e.g. 'questions'). High importance. |
| aura_notify_importance | While waiting for user input: set how urgent it is (0-100). Higher = more important. |
| aura_notify_progress | To show pipeline progress (0-100). Reset to low when a new plan/todo forms, then grow it. |

Rules:
- Notifications are best-effort; if unavailable, log a warning and continue.
- Always clear with aura_notify_off after obtaining the user's reply.
`

// ── helpers ─────────────────────────────────────────────────────────────────

func send(resp jsonrpcResponse) {
	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("marshal error: %v", err)
		return
	}
	fmt.Fprintln(os.Stdout, string(b))
}

func okResult(id json.RawMessage, text string) jsonrpcResponse {
	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: toolCallResult{
			Content: []contentItem{{Type: "text", Text: text}},
			IsError: false,
		},
	}
}

func errResult(id json.RawMessage, code int, msg string) jsonrpcResponse {
	return jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &jsonrpcError{Code: code, Message: msg},
	}
}

const resAuthorText = `# Author

Copyright (c) 2026 Vladislav Plotkin
SPDX-License-Identifier: MIT

This project is developed and maintained by Vladislav Plotkin.
`

func resourceListResult() interface{} {
	list := []resourceDef{
		{URI: resLcdURI, Name: "Display 16x2", Description: "2-line character display: layout, ASCII-only table, 16/32 limits, what to print.", MimeType: "text/markdown"},
		{URI: resNotificationsURI, Name: "Notification usage", Description: "When to call each notification tool (mandatory cases).", MimeType: "text/markdown"},
		{URI: resAuthorURI, Name: "Author info", Description: "Copyright and author information.", MimeType: "text/markdown"},
	}
	return map[string]interface{}{"resources": list}
}

func resourceRead(uri string) (string, bool) {
	switch uri {
	case resLcdURI:
		return resLcdText, true
	case resNotificationsURI:
		return resNotificationsText, true
	case resAuthorURI:
		return resAuthorText, true
	}
	return "", false
}

func toolListResult() interface{} {
	emptySchema := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	importanceSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"level": map[string]interface{}{
				"type":        "integer",
				"minimum":     0,
				"maximum":     100,
				"description": "Importance level, 0-100. Higher = more urgent/important.",
			},
		},
		"required": []string{"level"},
	}
	progressSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"percent": map[string]interface{}{
				"type":        "integer",
				"minimum":     0,
				"maximum":     100,
				"description": "Progress percentage, 0-100.",
			},
		},
		"required": []string{"percent"},
	}
	lcdSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Text to display (max 32 chars for full screen, 16 for single line).",
			},
			"row": map[string]interface{}{
				"type":        "integer",
				"description": "Row index (0 or 1) for single-line mode. Omit to use full screen (both lines).",
			},
		},
		"required": []string{"text"},
	}
	blinkSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"times": map[string]interface{}{
				"type":        "integer",
				"minimum":     1,
				"maximum":     20,
				"description": "Number of blink cycles (default: 3).",
			},
			"color": map[string]interface{}{
				"type":        "string",
				"description": "Optional blink color as 'R G B' (0-255 each). Default: '255 0 0' (red).",
			},
			"interval": map[string]interface{}{
				"type":        "integer",
				"minimum":     50,
				"maximum":     2000,
				"description": "On/off duration per half-cycle in ms (default: 300).",
			},
		},
	}
	list := []toolDef{
		{
			Name:        "aura_notify_start",
			Description: "Notify the user that a task/pipeline has STARTED. Call this as soon as work begins (mandatory).",
			InputSchema: emptySchema,
		},
		{
			Name:        "aura_notify_done",
			Description: "Notify the user that the WHOLE pipeline/task is FINISHED and verified. Call when all steps passed (mandatory).",
			InputSchema: emptySchema,
		},
		{
			Name:        "aura_notify_off",
			Description: "Turn the notification OFF. Call when a NEW task/instruction is received AND when a reply from the user has been received (mandatory — clear the notification once input is obtained).",
			InputSchema: emptySchema,
		},
		{
			Name:        "aura_notify_question",
			Description: "Notify the user that the agent is WAITING for their INPUT/ANSWER (e.g. via 'questions'). Call whenever the agent asks something and waits — this is mandatory. Set a high importance.",
			InputSchema: emptySchema,
		},
		{
			Name:        "aura_notify_importance",
			Description: "Set the IMPORTANCE of the notification (0-100). Higher = more urgent/important to the user, shown brighter. Use when waiting for user input: level reflects how critical the question is. Required arg: level (0-100).",
			InputSchema: importanceSchema,
		},
		{
			Name:        "aura_notify_progress",
			Description: "Show pipeline PROGRESS (0-100). Higher = more steps completed. When a new plan/todo is formed, reset to a low value and grow it as steps complete. Required arg: percent (0-100).",
			InputSchema: progressSchema,
		},
		{
			Name:        "aura_lcd_print",
			Description: "Print short text on the 2-line character display. 'row' selects line 0 or 1 (16 chars max, truncated with a warning if longer). If 'row' omitted, prints the FULL screen (both lines, 32 chars max, truncated with a warning). See aura-indicator://lcd for limits and what to print. Required arg: text. Optional: row (0 or 1).",
			InputSchema: lcdSchema,
		},
		{
			Name:        "aura_blink",
			Description: "Blink the indicator LED N times (on/off) WITHOUT disturbing the LCD, then restore the previous indicator color. Runs in the background and stops automatically. Optional: times (default 3), color 'R G B' (default '255 0 0'), interval ms (default 300). Use for attention-grabbing alerts.",
			InputSchema: blinkSchema,
		},
	}
	return map[string]interface{}{"tools": list}
}

// clampPct parses and clamps a percent value to 0-100.
func clampPct(v float64) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return int(v + 0.5)
}

// setIndicator quickly sets LED indicator via aura package.
func setIndicator(r, g, b byte, devicePath string) error {
	return aura.SetIndicatorDirect(r, g, b, devicePath)
}

// handleToolsCall dispatches a tools/call request.
func handleToolsCall(id json.RawMessage, params json.RawMessage) jsonrpcResponse {
	var p struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return errResult(id, -32602, "invalid params: "+err.Error())
	}

	// Fixed-state tools (no arguments).
	if tc, ok := tools[p.Name]; ok {
		stopBlink()
		if err := setIndicator(tc.r, tc.g, tc.b, ""); err != nil {
			return errResult(id, -32000, fmt.Sprintf("set indicator failed: %v", err))
		}
		return okResult(id, fmt.Sprintf("indicator set to (%d,%d,%d)", tc.r, tc.g, tc.b))
	}

	switch p.Name {
	case "aura_notify_importance":
		raw, ok := p.Arguments["level"]
		if !ok {
			return errResult(id, -32602, "missing required argument: level")
		}
		f, err := toFloat(raw)
		if err != nil {
			return errResult(id, -32602, "level must be a number 0-100")
		}
		level := clampPct(f)
		v := uint8(level * 255 / 100)
		stopBlink()
		if err := setIndicator(0, 0, v, ""); err != nil {
			return errResult(id, -32000, fmt.Sprintf("set indicator failed: %v", err))
		}
		return okResult(id, fmt.Sprintf("importance level=%d%% set", level))

	case "aura_notify_progress":
		raw, ok := p.Arguments["percent"]
		if !ok {
			return errResult(id, -32602, "missing required argument: percent")
		}
		f, err := toFloat(raw)
		if err != nil {
			return errResult(id, -32602, "percent must be a number 0-100")
		}
		percent := clampPct(f)
		v := uint8(percent * 255 / 100)
		stopBlink()
		if err := setIndicator(0, 0, v, ""); err != nil {
			return errResult(id, -32000, fmt.Sprintf("set indicator failed: %v", err))
		}
		return okResult(id, fmt.Sprintf("progress=%d%% set", percent))

	case "aura_lcd_print":
		text, ok := p.Arguments["text"].(string)
		if !ok {
			return errResult(id, -32602, "missing required argument: text")
		}

		if rawRow, ok := p.Arguments["row"]; ok {
			r, err := toInt(rawRow)
			if err != nil {
				return errResult(id, -32602, "row must be an integer 0 or 1")
			}
			if r != 0 && r != 1 {
				return errResult(id, -32602, "row must be 0 or 1")
			}
			warning, err := aura.SendLCDText(text, r, "")
			if err != nil {
				return errResult(id, -32000, fmt.Sprintf("lcd-line failed: %v", err))
			}
			return okResult(id, warning)
		} else {
			warning, err := aura.SendLCDText(text, -1, "")
			if err != nil {
				return errResult(id, -32000, fmt.Sprintf("lcd failed: %v", err))
			}
			return okResult(id, warning)
		}

	case "aura_blink":
		stopBlink()

		times := 3
		if raw, ok := p.Arguments["times"]; ok {
			t, err := toInt(raw)
			if err != nil {
				return errResult(id, -32602, "times must be an integer")
			}
			times = t
		}
		interval := 300
		if raw, ok := p.Arguments["interval"]; ok {
			i, err := toInt(raw)
			if err != nil {
				return errResult(id, -32602, "interval must be an integer (ms)")
			}
			interval = i
		}
		r, g, b := uint8(255), uint8(0), uint8(0)
		if raw, ok := p.Arguments["color"]; ok {
			c, ok := raw.(string)
			if !ok {
				return errResult(id, -32602, "color must be a string 'R G B'")
			}
			parts := strings.Fields(c)
			if len(parts) == 3 {
				rr, _ := strconv.Atoi(parts[0])
				gg, _ := strconv.Atoi(parts[1])
				bb, _ := strconv.Atoi(parts[2])
				r, g, b = uint8(rr), uint8(gg), uint8(bb)
			}
		}

		startBlink(r, g, b, times, interval, "")
		return okResult(id, fmt.Sprintf("blinking color=(%d,%d,%d) x%d times (interval %dms)", r, g, b, times, interval))
	}

	return errResult(id, -32601, "unknown tool: "+p.Name)
}

// toFloat coerces JSON numbers/strings to float64.
func toFloat(v interface{}) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case json.Number:
		return n.Float64()
	case string:
		return strconv.ParseFloat(n, 64)
	}
	return 0, fmt.Errorf("not a number: %v", v)
}

// toInt coerces JSON numbers/strings to int.
func toInt(v interface{}) (int, error) {
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case int:
		return n, nil
	case int64:
		return int(n), nil
	case json.Number:
		f, err := n.Int64()
		if err != nil {
			return 0, err
		}
		return int(f), nil
	case string:
		i, err := strconv.Atoi(n)
		if err != nil {
			return 0, err
		}
		return i, nil
	}
	return 0, fmt.Errorf("not an integer: %v", v)
}

// ── main loop ──────────────────────────────────────────────────────────────

func main() {
	log.SetOutput(os.Stderr) // keep stdout clean for JSON-RPC

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req jsonrpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			send(errResult(nil, -32700, "parse error"))
			continue
		}

		switch req.Method {
		case "initialize":
			send(jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities": map[string]interface{}{
						"tools":    map[string]interface{}{},
						"resources": map[string]interface{}{},
					},
					"serverInfo": map[string]interface{}{"name": "aura-indicator", "version": "1.2.0"},
				},
			})
		case "notifications/initialized":
			// no response
		case "ping":
			send(jsonrpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]interface{}{}})
		case "tools/list":
			send(jsonrpcResponse{JSONRPC: "2.0", ID: req.ID, Result: toolListResult()})
		case "tools/call":
			send(handleToolsCall(req.ID, req.Params))
		case "resources/list":
			send(jsonrpcResponse{JSONRPC: "2.0", ID: req.ID, Result: resourceListResult()})
		case "resources/read":
			var p struct {
				URI string `json:"uri"`
			}
			if err := json.Unmarshal(req.Params, &p); err != nil {
				send(errResult(req.ID, -32602, "invalid params: "+err.Error()))
				continue
			}
			text, ok := resourceRead(p.URI)
			if !ok {
				send(errResult(req.ID, -32602, "unknown resource uri: "+p.URI))
				continue
			}
			send(jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]interface{}{
					"contents": []map[string]interface{}{
						{"uri": p.URI, "mimeType": "text/markdown", "text": text},
					},
				},
			})
		default:
			if len(req.ID) > 0 {
				send(errResult(req.ID, -32601, "method not found: "+req.Method))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("scanner error: %v", err)
	}
}
