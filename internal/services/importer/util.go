package importer

import "time"

// ParseTimeStr tries to parse a date string in common formats.
// It reuses the parseTime function from rss.go.
func ParseTimeStr(s string) *time.Time {
	return parseTime(s)
}
