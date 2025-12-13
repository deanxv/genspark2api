package tooluse

import (
	"strings"
)

type ParserState int

const (
	StateInit ParserState = iota
	// JSON States
	StateInObject
	StateInKey
	StateColon
	StateInValue

	// Text States
	StateTextDetecting
	StateTextFindingTool
	StateTextReadingName
	StateTextReadingArgs
)

type StreamParser struct {
	cursor int

	// State
	state      ParserState
	currentKey string
	inString   bool
	isEscaped  bool

	// Hierarchy tracking
	stackDepth int

	// Detected fields
	ResponseType string
	ToolName     string

	// Buffering
	tempBuffer strings.Builder // for keys, values, and text buffering
	textBuffer strings.Builder // for accumulating full text prefix
}

type ParseEvent struct {
	Type    string // "content", "tool_call_inc", "tool_call_start"
	Content string
	Tool    string
}

func NewStreamParser() *StreamParser {
	return &StreamParser{
		stackDepth: 0,
		state:      StateInit,
	}
}

func (sp *StreamParser) Process(chunk string) ([]ParseEvent, error) {
	var events []ParseEvent

	for _, char := range chunk {
		// --- State Init Handling ---
		if sp.state == StateInit {
			if char == ' ' || char == '\n' || char == '\r' || char == '\t' {
				continue
			}
			if char == '{' {
				sp.state = StateInObject
				sp.stackDepth = 1
				continue
			} else {
				// Assume text format start
				sp.state = StateTextDetecting
				sp.textBuffer.WriteRune(char)
				// check if it matches prefix so far
				continue
			}
		}

		// --- Text Format Handling ---
		if sp.state >= StateTextDetecting {
			events = append(events, sp.processTextChar(char)...)
			continue
		}

		// --- JSON Format Handling ---
		prevDepth := sp.stackDepth

		// String State Handling
		if sp.inString {
			if sp.isEscaped {
				sp.isEscaped = false
				if sp.state == StateInKey {
					sp.tempBuffer.WriteRune(char)
				} else if sp.state == StateInValue {
					switch sp.currentKey {
					case "type", "tool":
						sp.tempBuffer.WriteRune(char)
					case "content":
						if sp.ResponseType == "response" {
							// Unescape logic
							var val string
							switch char {
							case 'n':
								val = "\n"
							case 'r':
								val = "\r"
							case 't':
								val = "\t"
							case '"':
								val = "\""
							case '\\':
								val = "\\"
							case '/':
								val = "/"
							case 'b':
								val = "\b"
							case 'f':
								val = "\f"
							default:
								val = "\\" + string(char)
							}
							events = append(events, ParseEvent{Type: "content", Content: val})
						} else if sp.ResponseType == "tool_call" {
							// For JSON tool calls, we might want to capture content too?
							// But based on existing logic, we only care about emitting raw JSON for arguments
						}
					}
				}
			} else {
				if char == '\\' {
					sp.isEscaped = true
				} else if char == '"' {
					sp.inString = false
					// End of string handling
					if sp.state == StateInKey {
						sp.currentKey = sp.tempBuffer.String()
						sp.tempBuffer.Reset()
						sp.state = StateColon
					} else if sp.state == StateInValue {
						if sp.currentKey == "type" {
							sp.ResponseType = sp.tempBuffer.String()
							sp.tempBuffer.Reset()
						}
						if sp.currentKey == "tool" {
							sp.ToolName = sp.tempBuffer.String()
							sp.tempBuffer.Reset()
						}
						sp.state = StateInObject
					}
				} else {
					// Normal char inside string
					if sp.state == StateInKey {
						sp.tempBuffer.WriteRune(char)
					} else if sp.state == StateInValue {
						switch sp.currentKey {
						case "type", "tool":
							sp.tempBuffer.WriteRune(char)
						case "content":
							if sp.ResponseType == "response" {
								events = append(events, ParseEvent{Type: "content", Content: string(char)})
							}
						}
					}
				}
			}
		} else {
			// Not Handle String
			switch char {
			case '{':
				sp.stackDepth++
				if sp.stackDepth == 1 {
					sp.state = StateInObject
				}
			case '}':
				sp.stackDepth--
				if sp.stackDepth == 1 {
					sp.state = StateInObject
				}
			case '"':
				sp.inString = true
				if sp.state == StateInObject {
					sp.state = StateInKey
				}
			case ':':
				if sp.state == StateColon {
					sp.state = StateInValue
				}
			case ',':
				if sp.state == StateInValue && sp.stackDepth == 1 {
					sp.state = StateInObject
					sp.currentKey = ""
				}
			}
		}

		// Emission Logic for Arguments (JSON mode)
		if sp.ResponseType == "tool_call" {
			shouldEmit := false
			if sp.stackDepth > 1 {
				shouldEmit = true
			} else if sp.stackDepth == 1 && prevDepth == 2 {
				shouldEmit = true
			}

			if shouldEmit && sp.ToolName != "" {
				events = append(events, ParseEvent{Type: "tool_call_inc", Content: string(char), Tool: sp.ToolName})
			}
		}
	}

	return events, nil
}

func (sp *StreamParser) processTextChar(char rune) []ParseEvent {
	var events []ParseEvent

	switch sp.state {
	case StateTextDetecting:
		sp.textBuffer.WriteRune(char)
		// We just accumulating until we hit a newline or something indicating tool list start
		// The prefix is "[Assistant called tools]:"
		// We can move to finding tool if we see '\n' or ':'
		if char == '\n' {
			sp.state = StateTextFindingTool
			sp.textBuffer.Reset()
		}

	case StateTextFindingTool:
		if char == '-' {
			sp.state = StateTextReadingName
			// consume space after dash if next char
		}
		// ignore other chars between tools

	case StateTextReadingName:
		if char == ' ' && sp.ToolName == "" {
			// skip leading space
			return nil
		}
		if char == '(' {
			sp.ToolName = sp.tempBuffer.String()
			sp.tempBuffer.Reset()
			sp.state = StateTextReadingArgs
			sp.ResponseType = "tool_call"
			// Emit tool name discovery if needed?
			// Current logic expects tool_call_inc logic to handle it
		} else {
			sp.tempBuffer.WriteRune(char)
		}

	case StateTextReadingArgs:
		// We are inside (...)
		// Arguments are JSON inside parens.
		if char == ')' && sp.stackDepth == 0 && !sp.inString {
			// End of args
			sp.state = StateTextFindingTool
			sp.ToolName = ""
			sp.ResponseType = ""
			return nil
		}

		// Track JSON structure to handle nested parens inside strings
		if sp.inString {
			if sp.isEscaped {
				sp.isEscaped = false
			} else {
				if char == '\\' {
					sp.isEscaped = true
				} else if char == '"' {
					sp.inString = false
				}
			}
		} else {
			if char == '"' {
				sp.inString = true
			} else if char == '{' {
				sp.stackDepth++
			} else if char == '}' {
				sp.stackDepth--
			}
		}

		events = append(events, ParseEvent{Type: "tool_call_inc", Content: string(char), Tool: sp.ToolName})
	}

	return events
}
