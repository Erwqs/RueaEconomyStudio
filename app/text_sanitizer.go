package app

import (
	"image/color"
	"strings"
	"time"

	"github.com/pkg/browser"
	"golang.design/x/clipboard"
)

// sanitizeTextValue trims whitespace and applies keyword replacements.
// It returns the sanitized value, whether any changes were made, and whether a banned phrase was found.
func sanitizeTextValue(value string) (string, bool, bool) {
	// sanitized := strings.TrimSpace(value)
	changed := false
	lower := strings.ToLower(value)
	flagged := false

	replacements := []struct {
		target      string
		replacement string
	}{
		{target: "new moon", replacement: "full moon"},
		{target: "newm", replacement: "fumo"},
	}

	for _, repl := range replacements {
		if strings.Contains(lower, repl.target) {
			flagged = true
			var replaced bool
			value, lower, replaced = replaceInsensitivePreserveCase(value, lower, repl.target, repl.replacement)
			changed = changed || replaced
		}
	}

	return value, changed, flagged
}

// replaceInsensitivePreserveCase swaps target occurrences regardless of case while leaving other characters untouched.
func replaceInsensitivePreserveCase(value, lowerValue, target, replacement string) (string, string, bool) {
	targetLower := strings.ToLower(target)
	if !strings.Contains(lowerValue, targetLower) {
		return value, lowerValue, false
	}

	var b strings.Builder
	start := 0
	replaced := false

	for {
		idx := strings.Index(lowerValue[start:], targetLower)
		if idx == -1 {
			break
		}
		idx += start
		b.WriteString(value[start:idx])
		b.WriteString(replacement)
		start = idx + len(target)
		replaced = true
	}

	b.WriteString(value[start:])
	newValue := b.String()
	return newValue, strings.ToLower(newValue), replaced
}

func showAdvertisingToast() {
	const inviteURL = "https://discord.gg/kEdaDnvHWD"

	toast := NewToast().
		AutoClose(5*time.Second).
		Text("Advertisment of a guild is not allowed, name has been changed to comply with this arbitrary rule", ToastOption{Colour: color.RGBA{R: 255, G: 127, B: 127, A: 255}}).
		Text("Consider joining HOC instead! :like:", ToastOption{Colour: color.RGBA{R: 127, G: 255, B: 127, A: 255}})

	toast.Button("Join HOC Discord", func() {
		_ = browser.OpenURL(inviteURL)
		clipboard.Write(clipboard.FmtText, []byte(inviteURL))
	}, 0, 0, ToastOption{})
	toast.Button("Copy Discord Link", func() {
		clipboard.Write(clipboard.FmtText, []byte(inviteURL))
	}, 0, 0, ToastOption{})

	toast.Show()
}
