// Package sample generates a realistic, seeded corporate card statement with
// a known answer key. It powers the demo button and the end-to-end tests:
// because we know exactly which subscriptions were planted, we can measure
// precision and recall instead of eyeballing output.
package sample

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"subscription-radar/internal/ledger"
)

// ExpectedSeries is a subscription planted in the statement.
type ExpectedSeries struct {
	Merchant string
	Cadence  string
	Status   string
}

// ExpectedAlert is a problem planted in the statement.
type ExpectedAlert struct {
	Kind     string
	Merchant string
	Date     string
}

type Statement struct {
	CSV    []byte
	Series []ExpectedSeries
	Alerts []ExpectedAlert
}

// DefaultSeed is what the demo uses, so the video and the tests match.
const DefaultSeed = 42

var (
	periodStart = ledger.MustParseDate("2025-09-01")
	periodEnd   = ledger.MustParseDate("2026-10-31")
)

type row struct {
	date        ledger.Date
	description string
	amount      ledger.Cents
}

type generator struct {
	rng  *rand.Rand
	rows []row
}

// Generate builds a 14-month statement for a small company's card: SaaS
// subscriptions mixed with travel, meals and supplies.
func Generate(seed uint64) Statement {
	g := &generator{rng: rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))}

	// --- Subscriptions (the answer key) ---------------------------------
	g.monthly(3, periodStart, periodEnd, func(d ledger.Date) (string, ledger.Cents) {
		// Seats were added in September: a price increase.
		if d.Before(ledger.MustParseDate("2026-09-01")) {
			return "SLACK*T" + g.ref(), 8750
		}
		return "SLACK*T" + g.ref(), 10500
	})
	g.monthly(21, periodStart, periodEnd, func(d ledger.Date) (string, ledger.Cents) {
		if d.Before(ledger.MustParseDate("2026-07-01")) {
			return "ADOBE *CREATIVE CLOUD", 5999
		}
		return "ADOBE *CREATIVE CLOUD", 6999
	})
	g.monthly(14, periodStart, periodEnd, fixed("ZOOM.US 888-799-9666 CA", 1499))
	g.add(ledger.MustParseDate("2026-08-15"), "ZOOM.US 888-799-9666 CA", 1499) // double billed
	g.monthly(9, periodStart, periodEnd, fixed("FIGMA INC SAN FRANCISCO CA", 4500))
	g.monthly(1, periodStart, periodEnd, fixed("GOOGLE *WORKSPACE", 7200))
	g.monthly(2, periodStart, periodEnd, func(ledger.Date) (string, ledger.Cents) {
		return "AMAZON WEB SERVICES AWS.AMAZON.COM WA", g.between(28000, 42000) // usage based
	})
	g.monthly(5, periodStart, periodEnd, fixed("GITHUB, INC. GITHUB.COM CA", 8400))
	g.monthly(27, periodStart, ledger.MustParseDate("2026-04-30"), func(ledger.Date) (string, ledger.Cents) {
		return "DROPBOX*" + g.ref(), 1999 // cancelled in May
	})
	for _, d := range []string{"2025-10-12", "2026-10-12"} {
		g.add(businessDay(ledger.MustParseDate(d)), "NOTION LABS, INC.", 96000)
	}
	for _, d := range []string{"2025-09-15", "2025-12-15", "2026-03-15", "2026-06-15", "2026-09-15"} {
		g.add(businessDay(ledger.MustParseDate(d)), "HUBSPOT INC", 27000)
	}
	for d := ledger.MustParseDate("2026-05-04"); !d.After(periodEnd); d = d.AddDays(7) {
		g.add(d, "SQ *BLUE BOTTLE COFFEE", 6400) // office coffee, every Monday
	}

	// --- Noise: real spending that must NOT be flagged -------------------
	restaurants := []string{"TST* JOE'S PIZZA", "SWEETGREEN NYC", "CHIPOTLE 2231", "DIG INN", "SHAKE SHACK #45", "BLUE RIBBON SUSHI", "PRET A MANGER", "NOBU NEW YORK"}
	stores := []string{"#10442", "#22871", "#30519"}
	for month := periodStart; !month.After(periodEnd); month = month.AddMonths(1) {
		for range g.rng.IntN(3) + 2 {
			g.add(g.dayIn(month), "UBER *TRIP HELP.UBER.COM CA", g.between(1200, 4800))
		}
		for range g.rng.IntN(3) + 1 {
			g.add(g.dayIn(month), "AMZN MKTP US*"+g.ref(), g.between(1500, 22000))
		}
		for range g.rng.IntN(4) + 3 {
			g.add(g.dayIn(month), "STARBUCKS STORE "+stores[g.rng.IntN(len(stores))], g.between(425, 980))
		}
		for range g.rng.IntN(3) + 3 {
			g.add(g.dayIn(month), restaurants[g.rng.IntN(len(restaurants))], g.between(2500, 14000))
		}
		// Office supplies about once a month: looks monthly, isn't a subscription.
		g.add(g.dayIn(month), "STAPLES 00123", g.between(2000, 18000))
		g.add(ledger.MustDate(month.Year(), month.Month(), 25), "PAYMENT RECEIVED - THANK YOU", -300000)
	}
	for d := periodStart.AddDays(g.rng.IntN(6)); !d.After(periodEnd); d = d.AddDays(5 + g.rng.IntN(8)) {
		g.add(d, "SHELL OIL 57444712", g.between(3800, 7200))
	}
	for range 3 {
		day := g.rng.IntN(periodStart.DaysUntil(periodEnd))
		g.add(periodStart.AddDays(day), "DELTA AIR LINES 0062345", g.between(18000, 62000))
		g.add(periodStart.AddDays(day+2), "MARRIOTT BOSTON COPLEY", g.between(32000, 90000))
	}
	g.add(ledger.MustParseDate("2026-03-18"), "AMZN MKTP US REFUND", -4599)

	return Statement{
		CSV: g.csv(),
		Series: []ExpectedSeries{
			{"Slack", "monthly", "active"},
			{"Adobe", "monthly", "active"},
			{"Zoom", "monthly", "active"},
			{"Figma", "monthly", "active"},
			{"Google Workspace", "monthly", "active"},
			{"Amazon Web Services", "monthly", "active"},
			{"GitHub", "monthly", "active"},
			{"Dropbox", "monthly", "inactive"},
			{"Notion Labs", "yearly", "active"},
			{"HubSpot", "quarterly", "active"},
			{"Blue Bottle Coffee", "weekly", "active"},
		},
		Alerts: []ExpectedAlert{
			{"duplicate", "Zoom", "2026-08-15"},
			{"price_increase", "Slack", "2026-09-03"},
			{"price_increase", "Adobe", "2026-07-21"},
		},
	}
}

func fixed(description string, amount ledger.Cents) func(ledger.Date) (string, ledger.Cents) {
	return func(ledger.Date) (string, ledger.Cents) { return description, amount }
}

// monthly bills on the anchor day (clamped to short months), posting weekend
// charges on the next Monday like real card networks do.
func (g *generator) monthly(anchorDay int, from, to ledger.Date, charge func(ledger.Date) (string, ledger.Cents)) {
	for month := ledger.MustDate(from.Year(), from.Month(), 1); !month.After(to); month = month.AddMonths(1) {
		day := min(anchorDay, daysIn(month))
		d := businessDay(ledger.MustDate(month.Year(), month.Month(), day))
		if d.Before(from) || d.After(to) {
			continue
		}
		description, amount := charge(d)
		g.add(d, description, amount)
	}
}

func (g *generator) add(d ledger.Date, description string, amount ledger.Cents) {
	if d.Before(periodStart) || d.After(periodEnd) {
		return
	}
	g.rows = append(g.rows, row{d, description, amount})
}

func (g *generator) dayIn(month ledger.Date) ledger.Date {
	return ledger.MustDate(month.Year(), month.Month(), 1+g.rng.IntN(daysIn(month)))
}

func (g *generator) between(lo, hi ledger.Cents) ledger.Cents {
	return lo + ledger.Cents(g.rng.Int64N(int64(hi-lo)))
}

// ref is a random reference id like the ones processors append after "*".
func (g *generator) ref() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = alphabet[g.rng.IntN(len(alphabet))]
	}
	// Guarantee digits so it looks like a real reference id.
	b[2], b[5] = byte('0'+g.rng.IntN(10)), byte('0'+g.rng.IntN(10))
	return string(b)
}

func (g *generator) csv() []byte {
	slices.SortStableFunc(g.rows, func(a, b row) int { return a.date.Compare(b.date) })
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"Date", "Description", "Amount"})
	for _, r := range g.rows {
		date := fmt.Sprintf("%02d/%02d/%04d", r.date.Month(), r.date.Day(), r.date.Year())
		_ = w.Write([]string{date, r.description, r.amount.String()})
	}
	w.Flush()
	return buf.Bytes()
}

func businessDay(d ledger.Date) ledger.Date {
	switch d.Weekday() {
	case time.Saturday:
		return d.AddDays(2)
	case time.Sunday:
		return d.AddDays(1)
	}
	return d
}

func daysIn(month ledger.Date) int {
	return ledger.MustDate(month.Year(), month.Month(), 1).AddMonths(1).AddDays(-1).Day()
}
