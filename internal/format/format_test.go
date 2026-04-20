package format

import (
	"strings"
	"testing"
)

// ── Currency ──────────────────────────────────────────────────────────────────

func TestCurrency(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "R$ 0,00"},
		{1, "R$ 1,00"},
		{1.5, "R$ 1,50"},
		{1000, "R$ 1.000,00"},
		{1234567.89, "R$ 1.234.567,89"},
		{-50.25, "-R$ 50,25"},
		{0.01, "R$ 0,01"},
		{999.999, "R$ 1.000,00"}, // rounds up
	}
	for _, c := range cases {
		got := Currency(c.in)
		if got != c.want {
			t.Errorf("Currency(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCurrency_negative(t *testing.T) {
	got := Currency(-1234.56)
	if !strings.HasPrefix(got, "-R$") {
		t.Errorf("negative currency should start with -R$, got %q", got)
	}
}

// ── DateDisplay ───────────────────────────────────────────────────────────────

func TestDateDisplay(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"2026-04-19", "19 Apr 2026"},
		{"2026-01-01", "01 Jan 2026"},
		{"2026-12-31", "31 Dec 2026"},
	}
	for _, c := range cases {
		got := DateDisplay(c.in)
		if got != c.want {
			t.Errorf("DateDisplay(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDateDisplay_invalid(t *testing.T) {
	// should return input unchanged
	got := DateDisplay("not-a-date")
	if got != "not-a-date" {
		t.Errorf("DateDisplay with invalid input should return input unchanged, got %q", got)
	}
}

// ── ParseDate ─────────────────────────────────────────────────────────────────

func TestParseDate_valid(t *testing.T) {
	got, err := ParseDate("2026-04-19")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "2026-04-19" {
		t.Errorf("got %q, want %q", got, "2026-04-19")
	}
}

func TestParseDate_withWhitespace(t *testing.T) {
	got, err := ParseDate("  2026-04-19  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "2026-04-19" {
		t.Errorf("got %q, want %q", got, "2026-04-19")
	}
}

func TestParseDate_invalid(t *testing.T) {
	cases := []string{"19/04/2026", "april 19", "", "2026-13-01", "abc"}
	for _, c := range cases {
		if _, err := ParseDate(c); err == nil {
			t.Errorf("ParseDate(%q) expected error, got nil", c)
		}
	}
}

// ── ParseFloat ────────────────────────────────────────────────────────────────

func TestParseFloat(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"1.50", 1.50},
		{"1,50", 1.50},   // comma separator
		{"1000", 1000},
		{"  42  ", 42},
		{"-3.14", -3.14},
	}
	for _, c := range cases {
		got, err := ParseFloat(c.in)
		if err != nil {
			t.Errorf("ParseFloat(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseFloat(%q) = %f, want %f", c.in, got, c.want)
		}
	}
}

func TestParseFloat_invalid(t *testing.T) {
	if _, err := ParseFloat("not-a-number"); err == nil {
		t.Error("expected error for non-numeric input")
	}
}

// ── PctDelta ──────────────────────────────────────────────────────────────────

func TestPctDelta(t *testing.T) {
	cases := []struct {
		cur, prev float64
		want      string
	}{
		{110, 100, "+10.0%"},
		{90, 100, "-10.0%"},
		{100, 0, "N/A"},   // prev=0 is undefined
		{0, 100, "-100.0%"},
		{150, 100, "+50.0%"},
	}
	for _, c := range cases {
		got := PctDelta(c.cur, c.prev)
		if got != c.want {
			t.Errorf("PctDelta(%v, %v) = %q, want %q", c.cur, c.prev, got, c.want)
		}
	}
}

func TestPctDelta_positiveHasPlus(t *testing.T) {
	got := PctDelta(200, 100)
	if !strings.HasPrefix(got, "+") {
		t.Errorf("positive delta should start with +, got %q", got)
	}
}

// ── TodayISO ──────────────────────────────────────────────────────────────────

func TestTodayISO_format(t *testing.T) {
	got := TodayISO()
	// must be parseable by ParseDate
	if _, err := ParseDate(got); err != nil {
		t.Errorf("TodayISO() returned unparseable date %q: %v", got, err)
	}
	// must look like YYYY-MM-DD
	if len(got) != 10 || got[4] != '-' || got[7] != '-' {
		t.Errorf("TodayISO() = %q, want YYYY-MM-DD format", got)
	}
}

// ── PeriodDates ───────────────────────────────────────────────────────────────

func TestPeriodDates_toIsToday(t *testing.T) {
	today := TodayISO()
	for _, mode := range []string{"1m", "3m", "6m", "1y"} {
		_, to := PeriodDates(mode)
		if to != today {
			t.Errorf("PeriodDates(%q).to = %q, want today %q", mode, to, today)
		}
	}
}

func TestPeriodDates_fromBeforeTo(t *testing.T) {
	for _, mode := range []string{"1m", "3m", "6m", "1y"} {
		from, to := PeriodDates(mode)
		if from >= to {
			t.Errorf("PeriodDates(%q): from %q >= to %q", mode, from, to)
		}
	}
}

func TestPeriodDates_unknownDefaultsTo1m(t *testing.T) {
	from1, to1 := PeriodDates("1m")
	fromX, toX := PeriodDates("unknown")
	if from1 != fromX || to1 != toX {
		t.Errorf("unknown mode should default to 1m: got %q–%q, want %q–%q", fromX, toX, from1, to1)
	}
}
