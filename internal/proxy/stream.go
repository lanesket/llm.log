package proxy

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/lanesket/llm.log/internal/provider"
)

// ParseSSE parses raw SSE data into events.
// SSE format: "event: <type>\ndata: <json>\n\n"
func ParseSSE(raw []byte) []provider.SSEEvent {
	var events []provider.SSEEvent
	scanner := bufio.NewScanner(bytes.NewReader(raw))

	var currentEvent string
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Empty line = end of event
			if len(dataLines) > 0 {
				data := strings.Join(dataLines, "\n")
				events = append(events, provider.SSEEvent{
					Event: currentEvent,
					Data:  []byte(data),
				})
			}
			currentEvent = ""
			dataLines = nil
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		} else if line == "data:" {
			dataLines = append(dataLines, "")
		}
	}

	// Handle trailing event without final newline
	if len(dataLines) > 0 {
		data := strings.Join(dataLines, "\n")
		events = append(events, provider.SSEEvent{
			Event: currentEvent,
			Data:  []byte(data),
		})
	}

	return events
}
