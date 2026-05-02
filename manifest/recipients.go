package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Recipient is one entry in recipients.json. Name is optional but strongly
// recommended for auditability — the whole point of keeping this file in git
// is to know who each key belongs to.
type Recipient struct {
	Name string `json:"name,omitempty"`
	Key  string `json:"key"`
	Note string `json:"note,omitempty"`
}

// RecipientsFile is the on-disk format. The top-level object form (with
// "recipients" key) is the canonical layout; LoadRecipients also accepts a
// bare JSON array of strings or objects for ergonomics.
type RecipientsFile struct {
	Recipients []Recipient `json:"recipients"`
}

// LoadRecipients reads path and returns its parsed entries. Returns
// (nil, nil) if path doesn't exist so callers can choose whether the
// absence is fatal.
//
// The file is parsed as JSONC: // line comments and /* */ block comments
// are stripped before JSON unmarshalling. The optional top-level "$schema"
// key (used by editors for validation) is ignored.
//
// Accepted shapes:
//
//	{"recipients": [{"name": "alice", "key": "age1..."}, ...]}
//	[{"name": "alice", "key": "age1..."}, ...]
//	["age1...", "age1..."]
func LoadRecipients(path string) ([]Recipient, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	data := stripJSONCComments(raw)
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}

	switch trimmed[0] {
	case '{':
		var f RecipientsFile
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		return validateRecipients(f.Recipients, path)
	case '[':
		// Distinguish array-of-objects vs array-of-strings by peeking at the
		// first non-whitespace byte after the opening bracket.
		if firstElementIsObject(trimmed) {
			var asObjects []Recipient
			if err := json.Unmarshal(data, &asObjects); err != nil {
				return nil, fmt.Errorf("parse %s: %w", path, err)
			}
			return validateRecipients(asObjects, path)
		}
		var asStrings []string
		if err := json.Unmarshal(data, &asStrings); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		out := make([]Recipient, 0, len(asStrings))
		for _, k := range asStrings {
			out = append(out, Recipient{Key: k})
		}
		return validateRecipients(out, path)
	default:
		return nil, fmt.Errorf("parse %s: expected JSON object or array", path)
	}
}

// Keys extracts just the age recipient strings, preserving order.
func Keys(rs []Recipient) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Key)
	}
	return out
}

// firstElementIsObject returns true if the first array element in s starts
// with '{', after skipping the opening '[' and whitespace. Used to decide
// whether to parse as []Recipient or []string.
func firstElementIsObject(s string) bool {
	if len(s) < 2 || s[0] != '[' {
		return false
	}
	for _, r := range s[1:] {
		switch r {
		case ' ', '\t', '\n', '\r':
			continue
		case '{':
			return true
		default:
			return false
		}
	}
	return false
}

// stripJSONCComments removes // line comments and /* */ block comments,
// leaving everything inside double-quoted strings (including escapes)
// untouched. The result remains positionally aligned where it matters
// (newlines preserved) so JSON parser error offsets stay sensible.
func stripJSONCComments(in []byte) []byte {
	out := make([]byte, 0, len(in))
	n := len(in)
	for i := 0; i < n; {
		if in[i] == '"' {
			// Copy the entire string literal verbatim.
			out = append(out, in[i])
			i++
			for i < n {
				if in[i] == '\\' && i+1 < n {
					out = append(out, in[i], in[i+1])
					i += 2
					continue
				}
				out = append(out, in[i])
				if in[i] == '"' {
					i++
					break
				}
				i++
			}
			continue
		}
		if i+1 < n && in[i] == '/' && in[i+1] == '/' {
			i += 2
			for i < n && in[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < n && in[i] == '/' && in[i+1] == '*' {
			i += 2
			for i+1 < n {
				if in[i] == '*' && in[i+1] == '/' {
					i += 2
					break
				}
				if in[i] == '\n' {
					out = append(out, '\n')
				}
				i++
			}
			continue
		}
		out = append(out, in[i])
		i++
	}
	return out
}

func validateRecipients(rs []Recipient, path string) ([]Recipient, error) {
	out := make([]Recipient, 0, len(rs))
	for i, r := range rs {
		key := strings.TrimSpace(r.Key)
		if key == "" {
			return nil, fmt.Errorf("%s: recipient #%d has empty key", path, i+1)
		}
		out = append(out, Recipient{Name: strings.TrimSpace(r.Name), Key: key, Note: strings.TrimSpace(r.Note)})
	}
	return out, nil
}
