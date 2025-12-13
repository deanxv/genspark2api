package tooluse

import (
	"strings"
)

type ParserState int

const (
	StateInit ParserState = iota
	// We are searching for a key in the main object
	StateInObject
	// We are reading a key string
	StateInKey
	// We have read a key, waiting for colon
	StateColon
	// We are reading a value
	StateInValue
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
	tempBuffer strings.Builder // for keys and "type" value
}

type ParseEvent struct {
	Type    string // "content" or "tool_call_inc"
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
		prevDepth := sp.stackDepth

		// --- String State Handling ---
		if sp.inString {
			if sp.isEscaped {
				sp.isEscaped = false

				// Handle specific captures
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
			// --- Not Handle String ---
			switch char {
			case '{':
				sp.stackDepth++
				if sp.stackDepth == 1 {
					sp.state = StateInObject
				}
			case '}':
				sp.stackDepth--
				if sp.stackDepth == 1 {
					// Closed inner object, back to outer object
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

		// --- Emission Logic for Arguments ---
		if sp.ResponseType == "tool_call" {
			// Rule: Everything inside arguments value is emitted as raw JSON
			// Arguments value is the ONLY object at depth > 1 (assuming simplistic tool call structure)
			shouldEmit := false

			if sp.stackDepth > 1 {
				shouldEmit = true
			} else if sp.stackDepth == 1 && prevDepth == 2 {
				// Just closed arguments object
				shouldEmit = true
			}

			if shouldEmit && sp.ToolName != "" {
				events = append(events, ParseEvent{Type: "tool_call_inc", Content: string(char), Tool: sp.ToolName})
			}
		}
	}

	return events, nil
}
