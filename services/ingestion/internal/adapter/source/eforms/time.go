package eforms

import "time"

// parseDeadline combines TED's separate date ("2026-08-11+03:00") and time
// ("15:00:00+03:00") strings into one time.Time. Both fields are
// fixed-width with the offset appended directly (verified live), so this
// slices by position rather than searching for delimiters — the date
// string's own "-" separators would otherwise be ambiguous with a
// negative UTC offset.
func parseDeadline(dateStr, timeStr string) *time.Time {
	if len(dateStr) < 10 {
		return nil
	}
	date := dateStr[:10]
	offset := "Z"
	if len(dateStr) > 10 {
		offset = dateStr[10:]
	}
	clock := "00:00:00"
	if len(timeStr) >= 8 {
		clock = timeStr[:8]
	}
	t, err := time.Parse(time.RFC3339, date+"T"+clock+offset)
	if err != nil {
		return nil
	}
	return &t
}
