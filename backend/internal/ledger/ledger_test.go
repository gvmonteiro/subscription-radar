package ledger

import "testing"

func TestAddMonthsClampsToMonthEnd(t *testing.T) {
	cases := []struct{ from, want string }{
		{"2026-01-31", "2026-02-28"},
		{"2028-01-31", "2028-02-29"}, // leap year
		{"2026-03-15", "2026-04-15"},
		{"2026-12-31", "2027-01-31"},
	}
	for _, c := range cases {
		if got := MustParseDate(c.from).AddMonths(1).String(); got != c.want {
			t.Errorf("%s + 1 month = %s, want %s", c.from, got, c.want)
		}
	}
}

func TestNewDateRejectsImpossibleDates(t *testing.T) {
	for _, d := range [][3]int{{2026, 2, 30}, {2026, 13, 1}, {2026, 4, 31}, {2026, 0, 10}} {
		if _, ok := NewDate(d[0], d[1], d[2]); ok {
			t.Errorf("%v should be invalid", d)
		}
	}
}

func TestDaysUntilAcrossDST(t *testing.T) {
	// Dates are UTC, so daylight-saving changes cannot produce 0.958-day gaps.
	if got := MustParseDate("2026-03-01").DaysUntil(MustParseDate("2026-04-01")); got != 31 {
		t.Errorf("got %d", got)
	}
}

func TestCentsJSON(t *testing.T) {
	cases := map[Cents]string{1299: "12.99", 5: "0.05", -4500: "-45.00", 0: "0.00", 123456789: "1234567.89"}
	for c, want := range cases {
		b, _ := c.MarshalJSON()
		if string(b) != want {
			t.Errorf("%d cents -> %s, want %s", int64(c), b, want)
		}
	}
	if FromFloat(0.1+0.2) != 30 {
		t.Error("float rounding should land on whole cents")
	}
}
