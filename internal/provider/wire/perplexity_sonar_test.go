package wire

import "testing"

func TestPerplexitySonar_Parse(t *testing.T) {
	body := []byte(`{
		"model": "sonar-pro",
		"usage": {
			"prompt_tokens": 20,
			"completion_tokens": 100,
			"total_tokens": 120
		},
		"citations": ["https://example.com"]
	}`)

	r, err := PerplexitySonar.Parse(body)
	if err != nil {
		t.Fatal(err)
	}
	if r.Model != "sonar-pro" {
		t.Errorf("model = %q", r.Model)
	}
	if r.InputTokens != 20 {
		t.Errorf("input = %d, want 20", r.InputTokens)
	}
	if r.OutputTokens != 100 {
		t.Errorf("output = %d, want 100", r.OutputTokens)
	}
}

func TestPerplexitySonar_ParseStream(t *testing.T) {
	events := []SSEEvent{
		{Data: []byte(`{"model":"sonar-pro","choices":[{"delta":{"content":"Hello"}}]}`)},
		{Data: []byte(`{"model":"sonar-pro","choices":[{"delta":{"content":" world"}}]}`)},
		{Data: []byte(`{"model":"sonar-pro","choices":[],"usage":{"prompt_tokens":15,"completion_tokens":2}}`)},
		{Data: []byte("[DONE]")},
	}

	r, err := PerplexitySonar.ParseStream(events)
	if err != nil {
		t.Fatal(err)
	}
	if r.Model != "sonar-pro" {
		t.Errorf("model = %q", r.Model)
	}
	if r.InputTokens != 15 {
		t.Errorf("input = %d, want 15", r.InputTokens)
	}
	if r.OutputTokens != 2 {
		t.Errorf("output = %d, want 2", r.OutputTokens)
	}
}

func TestPerplexitySonar_ModifyRequest(t *testing.T) {
	// Non-streaming request should not be modified.
	body := []byte(`{"model":"sonar-pro","messages":[]}`)
	result, _ := PerplexitySonar.ModifyRequest(body)
	if string(result) != string(body) {
		t.Error("non-streaming request was modified")
	}
}
