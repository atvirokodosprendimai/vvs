package jsutil

import "strings"

// EscapeJS escapes a string for safe embedding in a single-quoted JavaScript
// string literal. Escapes backslash, single quote, newlines, carriage returns,
// and </script> to prevent XSS and JS injection.
func EscapeJS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "'", `\'`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "</script>", `<\/script>`)
	return s
}
