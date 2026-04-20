package format

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Currency formats a float as Brazilian Real: R$ 1.234.567,89
func Currency(v float64) string {
	neg := v < 0
	v = math.Abs(v)
	intPart := int64(v)
	fracPart := int(math.Round((v - float64(intPart)) * 100))

	// format integer part with thousand separators using dots
	s := fmt.Sprintf("%d", intPart)
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, '.')
		}
		result = append(result, byte(c))
	}

	out := fmt.Sprintf("R$ %s,%02d", string(result), fracPart)
	if neg {
		out = "-" + out
	}
	return out
}

// DateDisplay converts YYYY-MM-DD to "19 Apr 2026"
func DateDisplay(d string) string {
	t, err := time.Parse("2006-01-02", d)
	if err != nil {
		return d
	}
	return t.Format("02 Jan 2006")
}

// ParseDate validates and normalises a date string to YYYY-MM-DD.
func ParseDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return "", fmt.Errorf("expected YYYY-MM-DD, got %q", s)
	}
	return t.Format("2006-01-02"), nil
}

// ParseFloat parses a decimal string accepting both "," and "." as decimal separator.
func ParseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", ".")
	return strconv.ParseFloat(s, 64)
}

// PctDelta computes percentage change from prev to cur, formatted as "+12.3%" / "-5.6%" / "N/A".
func PctDelta(cur, prev float64) string {
	if prev == 0 {
		return "N/A"
	}
	pct := (cur - prev) / math.Abs(prev) * 100
	if pct >= 0 {
		return fmt.Sprintf("+%.1f%%", pct)
	}
	return fmt.Sprintf("%.1f%%", pct)
}

// TodayISO returns today's date as YYYY-MM-DD.
func TodayISO() string {
	return time.Now().Format("2006-01-02")
}

// PeriodDates returns from/to dates for a named period mode relative to today.
func PeriodDates(mode string) (from, to string) {
	today := time.Now()
	to = today.Format("2006-01-02")
	switch mode {
	case "1m":
		from = today.AddDate(0, -1, 0).Format("2006-01-02")
	case "3m":
		from = today.AddDate(0, -3, 0).Format("2006-01-02")
	case "6m":
		from = today.AddDate(0, -6, 0).Format("2006-01-02")
	case "1y":
		from = today.AddDate(-1, 0, 0).Format("2006-01-02")
	default:
		from = today.AddDate(0, -1, 0).Format("2006-01-02")
	}
	return
}
