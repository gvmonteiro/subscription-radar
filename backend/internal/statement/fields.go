package statement

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"subscription-radar/internal/ledger"
)

type dateOrder int

const (
	monthFirst dateOrder = iota
	dayFirst
)

var (
	isoDateRe   = regexp.MustCompile(`^(\d{4})[-/.](\d{1,2})[-/.](\d{1,2})$`)
	shortDateRe = regexp.MustCompile(`^(\d{1,2})[-/.](\d{1,2})[-/.](\d{2}|\d{4})$`)
	// Textual formats some banks use: "Mar 3, 2026", "03-Mar-2026".
	textDateLayouts = []string{"Jan 2, 2006", "Jan 2 2006", "2 Jan 2006", "02-Jan-2006", "January 2, 2006", "2 January 2006", "02-Jan-06"}
)

// detectDateOrder looks at the whole column instead of one value: a first
// part above 12 can only be a day, a second part above 12 can only be a day.
func detectDateOrder(raw []string) (order dateOrder, ambiguous bool) {
	var firstOver12, secondOver12, sawShort bool
	for _, r := range raw {
		m := shortDateRe.FindStringSubmatch(datePart(r))
		if m == nil {
			continue
		}
		sawShort = true
		if atoi(m[1]) > 12 {
			firstOver12 = true
		}
		if atoi(m[2]) > 12 {
			secondOver12 = true
		}
	}
	switch {
	case firstOver12 && !secondOver12:
		return dayFirst, false
	case secondOver12 && !firstOver12:
		return monthFirst, false
	default:
		return monthFirst, sawShort && !firstOver12 && !secondOver12
	}
}

func parseDate(raw string, order dateOrder) (ledger.Date, bool) {
	value := datePart(raw)
	if m := isoDateRe.FindStringSubmatch(value); m != nil {
		return ledger.NewDate(atoi(m[1]), atoi(m[2]), atoi(m[3]))
	}
	if m := shortDateRe.FindStringSubmatch(value); m != nil {
		a, b, year := atoi(m[1]), atoi(m[2]), atoi(m[3])
		if year < 100 {
			year += 2000
		}
		if order == dayFirst {
			return ledger.NewDate(year, b, a)
		}
		return ledger.NewDate(year, a, b)
	}
	trimmed := strings.TrimSpace(raw)
	for _, layout := range textDateLayouts {
		if t, err := time.Parse(layout, trimmed); err == nil {
			return ledger.NewDate(t.Year(), int(t.Month()), t.Day())
		}
	}
	return ledger.Date{}, false
}

// datePart drops a time component: "2026-03-01 10:22" or "2026-03-01T10:22:00Z".
func datePart(raw string) string {
	s := strings.TrimSpace(raw)
	if i := strings.IndexAny(s, " T"); i >= 0 {
		return s[:i]
	}
	return s
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// ---------------------------------------------------------------------------
// Amounts
// ---------------------------------------------------------------------------

var decimalCommaTail = regexp.MustCompile(`,\d{1,2}$`)

// parseAmount understands "$1,234.56", "R$ 1.234,56", "(12.99)", "12.99-",
// "-€5" and friends. The last separator followed by 1-2 digits is the decimal
// separator; any other separator is a thousands separator.
func parseAmount(raw string) (ledger.Cents, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, false
	}
	negative := false
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		negative = true
		s = s[1 : len(s)-1]
	}
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9', r == '.', r == ',', r == '-':
			return r
		case r == '−': // unicode minus
			return '-'
		default:
			return -1 // currency symbols, codes, spaces
		}
	}, s)
	if strings.HasPrefix(s, "-") {
		negative = !negative
		s = s[1:]
	}
	if strings.HasSuffix(s, "-") {
		negative = !negative
		s = s[:len(s)-1]
	}
	if s == "" || strings.Contains(s, "-") || !strings.ContainsAny(s, "0123456789") {
		return 0, false
	}

	lastComma, lastDot := strings.LastIndex(s, ","), strings.LastIndex(s, ".")
	var decimal string
	switch {
	case lastComma >= 0 && lastDot >= 0:
		if lastComma > lastDot {
			decimal = ","
		} else {
			decimal = "."
		}
	case lastComma >= 0:
		if decimalCommaTail.MatchString(s) {
			decimal = ","
		}
	case lastDot >= 0:
		if strings.Count(s, ".") == 1 {
			decimal = "."
		}
	}

	var normalized string
	if decimal == "" {
		normalized = strings.NewReplacer(".", "", ",", "").Replace(s)
	} else {
		thousands := ","
		if decimal == "," {
			thousands = "."
		}
		normalized = strings.Replace(strings.ReplaceAll(s, thousands, ""), decimal, ".", 1)
	}

	f, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}
	c := ledger.FromFloat(f)
	if negative {
		c = -c
	}
	return c, true
}

var currencyCodeRe = regexp.MustCompile(`\b([A-Z]{3})\b`)

// currencyFromSymbol guesses the currency from the raw amount text.
func currencyFromSymbol(raw string) string {
	switch {
	case strings.Contains(raw, "R$"):
		return "BRL"
	case strings.Contains(raw, "MX$"):
		return "MXN"
	case strings.Contains(raw, "CA$"):
		return "CAD"
	case strings.Contains(raw, "€"):
		return "EUR"
	case strings.Contains(raw, "£"):
		return "GBP"
	}
	if m := currencyCodeRe.FindStringSubmatch(raw); m != nil {
		return m[1]
	}
	if strings.Contains(raw, "$") {
		return "USD"
	}
	return ""
}
