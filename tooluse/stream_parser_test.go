package tooluse

import (
	"strings"
	"testing"
)

func TestStreamParser_Process_Content(t *testing.T) {
	parser := NewStreamParser()

	chunks := []string{
		`{"type"`,
		`:"response",`,
		`"content":"Hello`,
		` World!"}`,
	}

	expected := []string{"H", "e", "l", "l", "o", " ", "W", "o", "r", "l", "d", "!"}
	var actual []string

	for _, chunk := range chunks {
		events, err := parser.Process(chunk)
		if err != nil {
			t.Fatalf("Process error: %v", err)
		}
		for _, e := range events {
			if e.Type == "content" {
				// We might get multiple chars in one event or multiple events
				// Actually current impl sends one char per event mostly
				actual = append(actual, e.Content)
			}
		}
	}

	fullActual := strings.Join(actual, "")
	fullExpected := strings.Join(expected, "")

	if fullActual != fullExpected {
		t.Errorf("Expected content %q, got %q", fullExpected, fullActual)
	}
}

func TestStreamParser_Process_ToolCall(t *testing.T) {
	parser := NewStreamParser()

	chunks := []string{
		`{"type":"tool_call","tool":"get_weather","arguments":{"ci`,
		`ty":"Paris"}}`,
	}

	var actualArgs strings.Builder
	var toolName string

	for _, chunk := range chunks {
		events, err := parser.Process(chunk)
		if err != nil {
			t.Fatalf("Process error: %v", err)
		}
		for _, e := range events {
			if e.Type == "tool_call_inc" {
				actualArgs.WriteString(e.Content)
				if e.Tool != "" {
					toolName = e.Tool
				}
			}
		}
	}

	if toolName != "get_weather" {
		t.Errorf("Expected tool name get_weather, got %q", toolName)
	}

	expectedArgs := `{"city":"Paris"}`
	if actualArgs.String() != expectedArgs {
		t.Errorf("Expected args %q, got %q", expectedArgs, actualArgs.String())
	}
}

func TestStreamParser_Process_Escaped(t *testing.T) {
	parser := NewStreamParser()

	// Test escaped quote and newline
	input := `{"type":"response","content":"Line 1\nLine \"2\""}`

	events, _ := parser.Process(input)
	var content strings.Builder
	for _, e := range events {
		if e.Type == "content" {
			content.WriteString(e.Content)
		}
	}

	expected := "Line 1\nLine \"2\""
	if content.String() != expected {
		t.Errorf("Expected content %q, got %q", expected, content.String())
	}
}

func TestStreamParser_Process_TextToolCall(t *testing.T) {
	parser := NewStreamParser()

	// Text format: [Assistant called tools]:\n- tool_name({"arg":"val"})\n
	chunks := []string{
		"[Assistant called tools]:\n",
		"- read_file",
		"(",
		`{"path":"test.go"}`,
		")\n",
	}

	var actualArgs strings.Builder
	var toolName string

	for _, chunk := range chunks {
		events, err := parser.Process(chunk)
		if err != nil {
			t.Fatalf("Process error: %v", err)
		}
		for _, e := range events {
			if e.Type == "tool_call_inc" {
				actualArgs.WriteString(e.Content)
				if e.Tool != "" {
					toolName = e.Tool
				}
			}
		}
	}

	if toolName != "read_file" {
		t.Errorf("Expected tool name read_file, got %q", toolName)
	}

	expectedArgs := `{"path":"test.go"}`
	if actualArgs.String() != expectedArgs {
		t.Errorf("Expected args %q, got %q", expectedArgs, actualArgs.String())
	}
}
