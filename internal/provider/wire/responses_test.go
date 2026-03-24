package wire

import (
	"encoding/json"
	"testing"
)

func TestResponses_Parse(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4.1",
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50,
			"input_token_details": {"cached_tokens": 20}
		}
	}`)

	r, err := Responses.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if r.Model != "gpt-4.1" {
		t.Errorf("model = %q", r.Model)
	}
	if r.InputTokens != 100 {
		t.Errorf("input = %d, want 100", r.InputTokens)
	}
	if r.OutputTokens != 50 {
		t.Errorf("output = %d, want 50", r.OutputTokens)
	}
	if r.CacheReadTokens != 20 {
		t.Errorf("cached = %d, want 20", r.CacheReadTokens)
	}
}

func TestResponses_ParseStream(t *testing.T) {
	events := []SSEEvent{
		{Event: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`)},
		{Event: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","delta":"Hello"}`)},
		{Event: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","delta":" world"}`)},
		{Event: "response.completed", Data: []byte(`{"type":"response.completed","response":{"model":"gpt-4.1","usage":{"input_tokens":100,"output_tokens":2,"input_token_details":{"cached_tokens":30}}}}`)},
	}

	r, err := Responses.ParseStream(events)
	if err != nil {
		t.Fatal(err)
	}
	if r.Model != "gpt-4.1" {
		t.Errorf("model = %q", r.Model)
	}
	if r.InputTokens != 100 {
		t.Errorf("input = %d, want 100", r.InputTokens)
	}
	if r.OutputTokens != 2 {
		t.Errorf("output = %d, want 2", r.OutputTokens)
	}
	if r.CacheReadTokens != 30 {
		t.Errorf("cached = %d, want 30", r.CacheReadTokens)
	}

	var body map[string]any
	json.Unmarshal(r.ResponseBody, &body)
	if body["content"] != "Hello world" {
		t.Errorf("content = %q", body["content"])
	}
}

func TestResponses_Parse_AudioTokens(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o-audio-preview",
		"usage": {
			"input_tokens": 1000,
			"output_tokens": 500,
			"input_token_details": {"cached_tokens": 0, "audio_tokens": 600},
			"output_token_details": {"audio_tokens": 250}
		}
	}`)

	r, err := Responses.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if r.AudioInputTokens != 600 {
		t.Errorf("audio input = %d, want 600", r.AudioInputTokens)
	}
	if r.AudioOutputTokens != 250 {
		t.Errorf("audio output = %d, want 250", r.AudioOutputTokens)
	}
}

func TestResponses_ParseStream_AudioTokens(t *testing.T) {
	events := []SSEEvent{
		{Event: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","delta":"Hi"}`)},
		{Event: "response.completed", Data: []byte(`{"type":"response.completed","response":{"model":"gpt-4o-audio","usage":{"input_tokens":500,"output_tokens":200,"input_token_details":{"cached_tokens":0,"audio_tokens":300},"output_token_details":{"audio_tokens":100}}}}`)},
	}

	r, err := Responses.ParseStream(events)
	if err != nil {
		t.Fatal(err)
	}
	if r.AudioInputTokens != 300 {
		t.Errorf("audio input = %d, want 300", r.AudioInputTokens)
	}
	if r.AudioOutputTokens != 100 {
		t.Errorf("audio output = %d, want 100", r.AudioOutputTokens)
	}
}

func TestResponses_ModifyRequest_Passthrough(t *testing.T) {
	body := []byte(`{"model":"gpt-4.1","input":[]}`)
	result, err := Responses.ModifyRequest(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != string(body) {
		t.Error("Responses API should not modify requests")
	}
}
