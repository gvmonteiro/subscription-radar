// Package ledger holds the small domain types shared by the parser and the
// detector: calendar dates, money in cents and a normalised transaction.
package ledger

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

// Date is a calendar day with no time zone. It is stored as UTC midnight so
// day arithmetic is exact.
type Date struct{ t time.Time }

// NewDate validates the day (no 02/30) and returns false for impossible dates.
func NewDate(year, month, day int) (Date, bool) {
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return Date{}, false
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if t.Day() != day {
		return Date{}, false
	}
	return Date{t}, true
}

// MustDate is NewDate for values known to be valid. For tests and fixtures.
func MustDate(year, month, day int) Date {
	d, ok := NewDate(year, month, day)
	if !ok {
		panic(fmt.Sprintf("invalid date %04d-%02d-%02d", year, month, day))
	}
	return d
}

// MustParseDate parses YYYY-MM-DD and panics on bad input. For tests and fixtures.
func MustParseDate(s string) Date {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return Date{t}
}

func (d Date) IsZero() bool                 { return d.t.IsZero() }
func (d Date) String() string               { return d.t.Format(time.DateOnly) }
func (d Date) Year() int                    { return d.t.Year() }
func (d Date) Month() int                   { return int(d.t.Month()) }
func (d Date) Day() int                     { return d.t.Day() }
func (d Date) Weekday() time.Weekday        { return d.t.Weekday() }
func (d Date) Compare(o Date) int           { return d.t.Compare(o.t) }
func (d Date) Before(o Date) bool           { return d.t.Before(o.t) }
func (d Date) After(o Date) bool            { return d.t.After(o.t) }
func (d Date) AddDays(n int) Date           { return Date{d.t.AddDate(0, 0, n)} }
func (d Date) DaysUntil(o Date) int         { return int(math.Round(o.t.Sub(d.t).Hours() / 24)) }
func (d Date) MarshalJSON() ([]byte, error) { return []byte(`"` + d.String() + `"`), nil }

func (d *Date) UnmarshalJSON(b []byte) error {
	t, err := time.Parse(`"`+time.DateOnly+`"`, string(b))
	if err != nil {
		return err
	}
	*d = Date{t}
	return nil
}

// AddMonths moves by calendar months, clamping to the end of shorter months
// (Jan 31 + 1 month = Feb 28), which is how billing systems renew.
func (d Date) AddMonths(n int) Date {
	first := time.Date(d.t.Year(), d.t.Month()+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
	lastDay := first.AddDate(0, 1, -1).Day()
	return Date{first.AddDate(0, 0, min(d.t.Day(), lastDay)-1)}
}

// Cents is an amount of money in minor units. Integer maths keeps totals exact.
type Cents int64

// FromFloat converts a decimal amount such as 12.99 into cents.
func FromFloat(f float64) Cents { return Cents(math.Round(f * 100)) }

func (c Cents) Float() float64 { return float64(c) / 100 }

func (c Cents) String() string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	return fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
}

// MarshalJSON writes a plain JSON number in major units (12.99), which is what
// the frontend formats with Intl.NumberFormat.
func (c Cents) MarshalJSON() ([]byte, error) { return []byte(c.String()), nil }

func (c *Cents) UnmarshalJSON(b []byte) error {
	f, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return err
	}
	*c = FromFloat(f)
	return nil
}

// Transaction is one charge (money out) from the statement.
type Transaction struct {
	// Row is the line number in the source file, so users can find it again.
	Row         int    `json:"row"`
	Date        Date   `json:"date"`
	Description string `json:"description"`
	// Amount is always positive: money spent.
	Amount   Cents  `json:"amount"`
	Currency string `json:"currency"`
}
