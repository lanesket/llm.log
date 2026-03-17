package proxy

import (
	"testing"
)

func TestParseSSE(t *testing.T) {
	raw := []byte("event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: content_block_delta\ndata: {\"delta\":{}}\n\ndata: [DONE]\n\n")
	events := ParseSSE(raw)

	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	if events[0].Event != "message_start" {
		t.Errorf("event[0].Event = %q, want message_start", events[0].Event)
	}
	if string(events[0].Data) != `{"type":"message_start"}` {
		t.Errorf("event[0].Data = %q", events[0].Data)
	}
	if events[1].Event != "content_block_delta" {
		t.Errorf("event[1].Event = %q, want content_block_delta", events[1].Event)
	}
	if string(events[2].Data) != "[DONE]" {
		t.Errorf("event[2].Data = %q, want [DONE]", events[2].Data)
	}
}

func TestParseSSE_NoEventField(t *testing.T) {
	raw := []byte("data: {\"choices\":[]}\n\ndata: {\"usage\":{}}\n\n")
	events := ParseSSE(raw)

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	for _, ev := range events {
		if ev.Event != "" {
			t.Errorf("expected empty event, got %q", ev.Event)
		}
	}
}

func TestParseSSE_NoTrailingNewline(t *testing.T) {
	raw := []byte("data: {\"test\":true}")
	events := ParseSSE(raw)

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
}

func TestParseSSE_MultilineData(t *testing.T) {
	raw := []byte("data: line1\ndata: line2\n\n")
	events := ParseSSE(raw)

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if string(events[0].Data) != "line1\nline2" {
		t.Errorf("data = %q, want line1\\nline2", events[0].Data)
	}
}

func TestParseSSE_EmptyDataLine(t *testing.T) {
	raw := []byte("data:\n\n")
	events := ParseSSE(raw)

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if string(events[0].Data) != "" {
		t.Errorf("data = %q, want empty", events[0].Data)
	}
}

func TestParseSSE_IgnoresComments(t *testing.T) {
	raw := []byte(": this is a comment\ndata: hello\n\n")
	events := ParseSSE(raw)

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if string(events[0].Data) != "hello" {
		t.Errorf("data = %q, want hello", events[0].Data)
	}
}
