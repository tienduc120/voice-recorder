package domain

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	// startLayout sorts names in date order and keeps the minute the owner remembers the meeting by.
	startLayout   = "2006-01-02_1504"
	fileExt       = ".ogg"
	titleSep      = '_'
	attemptSep    = '-'
	firstAttempt  = 1
	maxTitleRunes = 60
	// dash replaces every character a title may not keep, so runs of them read as one separator.
	dash = '-'
	// trimmed are the characters a title must not start or end with: Windows drops a trailing dot, and the
	// others would only stick to the separators around the title.
	trimmed = "-._"
)

// FileName names a recording after its local start time and a cleaned title. Attempt 2, 3, … is appended when an
// earlier attempt's name was taken.
func FileName(start time.Time, title string, attempt int) string {
	var b strings.Builder
	b.WriteString(start.Format(startLayout))
	if t := cleanTitle(title); t != "" {
		b.WriteRune(titleSep)
		b.WriteString(t)
	}
	if attempt > firstAttempt {
		b.WriteRune(attemptSep)
		b.WriteString(strconv.Itoa(attempt))
	}
	b.WriteString(fileExt)
	return b.String()
}

// cleanTitle keeps letters, digits and combining marks in any script. Everything else, which covers every
// character Windows forbids in a file name, becomes a dash. The date prefix means no reserved name can result.
func cleanTitle(raw string) string {
	var b strings.Builder
	lastWasDash := false
	for _, r := range raw {
		if keepInTitle(r) {
			b.WriteRune(r)
			lastWasDash = false
			continue
		}
		if !lastWasDash {
			b.WriteRune(dash)
			lastWasDash = true
		}
	}
	title := strings.Trim(b.String(), trimmed)
	if runes := []rune(title); len(runes) > maxTitleRunes {
		title = strings.Trim(string(runes[:maxTitleRunes]), trimmed)
	}
	return title
}

func keepInTitle(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) || r == '_' || r == '.'
}
