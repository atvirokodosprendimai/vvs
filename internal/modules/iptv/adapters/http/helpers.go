package http

import "encoding/json"

// jsStr encodes s as a JSON string literal (e.g. `"foo"`) safe for embedding
// inside Datastar data-on:click expressions.
func jsStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
