package recurring

import (
	"testing"

	"subscription-radar/internal/ledger"
)

// builder makes readable fixtures: b.every(30, 12, "NETFLIX.COM", 1549).
type builder struct {
	start ledger.Date
	txs   []ledger.Transaction
}

func newBuilder() *builder { return &builder{start: ledger.MustParseDate("2025-01-01")} }

func (b *builder) add(date, description string, amount ledger.Cents) *builder {
	return b.addTx(ledger.MustParseDate(date), description, amount, "USD")
}

func (b *builder) addTx(date ledger.Date, description string, amount ledger.Cents, currency string) *builder {
	b.txs = append(b.txs, ledger.Transaction{Row: len(b.txs) + 2, Date: date, Description: description, Amount: amount, Currency: currency})
	return b
}

// monthly adds n charges on the same day of consecutive months.
func (b *builder) monthly(n int, description string, amount func(i int) ledger.Cents) *builder {
	for i := range n {
		b.addTx(b.start.AddMonths(i), description, amount(i), "USD")
	}
	return b
}

func price(c ledger.Cents) func(int) ledger.Cents { return func(int) ledger.Cents { return c } }

func onlySeries(t *testing.T, a Analysis) Series {
	t.Helper()
	if len(a.Series) != 1 {
		t.Fatalf("want exactly 1 series, got %d: %+v", len(a.Series), a.Series)
	}
	return a.Series[0]
}

func TestMonthlyFixedSubscription(t *testing.T) {
	a := Analyze(newBuilder().monthly(12, "NETFLIX.COM", price(1549)).txs)
	s := onlySeries(t, a)
	if s.Merchant != "Netflix" || s.Cadence != Monthly || s.Pricing != PricingFixed || s.Status != StatusActive || s.Confidence != ConfidenceHigh {
		t.Errorf("unexpected series: %+v", s)
	}
	if s.AnnualCost != 1549*12 {
		t.Errorf("annual cost = %v", s.AnnualCost)
	}
	if s.NextExpected == nil || s.NextExpected.String() != "2026-01-01" {
		t.Errorf("next expected = %v", s.NextExpected)
	}
	if len(a.Alerts) != 0 {
		t.Errorf("unexpected alerts: %+v", a.Alerts)
	}
}

func TestEachCadence(t *testing.T) {
	cases := []struct {
		cadence Cadence
		gapDays int
		count   int
	}{
		{Weekly, 7, 8},
		{Biweekly, 14, 6},
		{Quarterly, 91, 4},
		{Yearly, 365, 2},
	}
	for _, c := range cases {
		b := newBuilder()
		for i := range c.count {
			b.addTx(b.start.AddDays(i*c.gapDays), "SERVICE", 1000, "USD")
		}
		s := onlySeries(t, Analyze(b.txs))
		if s.Cadence != c.cadence {
			t.Errorf("gap %d: got %s, want %s", c.gapDays, s.Cadence, c.cadence)
		}
	}
}

func TestToleratesMonthLengthAndOneMissedCharge(t *testing.T) {
	// Real postings: weekend shifts and a skipped month.
	b := newBuilder().
		add("2025-01-31", "FIGMA", 4500).
		add("2025-02-28", "FIGMA", 4500).
		add("2025-03-31", "FIGMA", 4500).
		add("2025-05-02", "FIGMA", 4500). // April missed, May posted late
		add("2025-06-02", "FIGMA", 4500).
		add("2025-06-30", "FIGMA", 4500)
	s := onlySeries(t, Analyze(b.txs))
	if s.Cadence != Monthly {
		t.Errorf("got %s", s.Cadence)
	}
}

func TestNotRecurring(t *testing.T) {
	cases := map[string]*builder{
		"only two monthly charges": newBuilder().monthly(2, "FIGMA", price(4500)),
		"irregular gaps": newBuilder().
			add("2025-01-01", "STAPLES", 5000).add("2025-01-09", "STAPLES", 5000).
			add("2025-02-20", "STAPLES", 5000).add("2025-03-01", "STAPLES", 5000).
			add("2025-04-28", "STAPLES", 5000),
		"regular visits, erratic amounts": newBuilder().
			add("2025-01-06", "WHOLE FOODS", 4210).add("2025-01-13", "WHOLE FOODS", 12890).
			add("2025-01-20", "WHOLE FOODS", 2350).add("2025-01-27", "WHOLE FOODS", 9875).
			add("2025-02-03", "WHOLE FOODS", 15420),
		"single charge": newBuilder().add("2025-01-01", "DELTA", 45000),
	}
	for name, b := range cases {
		if a := Analyze(b.txs); len(a.Series) != 0 {
			t.Errorf("%s: should not be recurring, got %+v", name, a.Series)
		}
	}
}

func TestDuplicateCharge(t *testing.T) {
	b := newBuilder().monthly(6, "ZOOM.US", price(1499)).add("2025-03-02", "ZOOM.US", 1499)
	a := Analyze(b.txs)
	s := onlySeries(t, a)
	if len(s.Charges) != 6 || len(s.Duplicates) != 1 {
		t.Fatalf("charges=%d duplicates=%d", len(s.Charges), len(s.Duplicates))
	}
	if len(a.Alerts) != 1 || a.Alerts[0].Kind != AlertDuplicate || a.Alerts[0].Date.String() != "2025-03-02" || a.Alerts[0].OriginalDate.String() != "2025-03-01" {
		t.Fatalf("unexpected alerts: %+v", a.Alerts)
	}
	if a.Totals[0].Refundable != 1499 {
		t.Errorf("refundable = %v", a.Totals[0].Refundable)
	}
	if s.TotalPaid != 1499*7 {
		t.Errorf("total paid should include the duplicate, got %v", s.TotalPaid)
	}
}

func TestSameAmountFarApartIsNotDuplicate(t *testing.T) {
	b := newBuilder().monthly(6, "ZOOM.US", price(1499)).add("2025-03-10", "ZOOM.US", 1499)
	if a := Analyze(b.txs); len(a.Alerts) != 0 {
		t.Errorf("9 days apart is not double billing: %+v", a.Alerts)
	}
}

func TestPriceIncrease(t *testing.T) {
	b := newBuilder().monthly(12, "ADOBE *CREATIVE CLOUD", func(i int) ledger.Cents {
		if i < 9 {
			return 5999
		}
		return 6999
	})
	a := Analyze(b.txs)
	s := onlySeries(t, a)
	if s.PriceIncreasedOn == nil || s.PriceIncreasedOn.String() != "2025-10-01" {
		t.Errorf("price increased on = %v", s.PriceIncreasedOn)
	}
	if s.AnnualCost != 6999*12 {
		t.Errorf("annual cost should use the new price, got %v", s.AnnualCost)
	}
	if len(a.Alerts) != 1 {
		t.Fatalf("want 1 alert, got %+v", a.Alerts)
	}
	alert := a.Alerts[0]
	if alert.Kind != AlertPriceIncrease || alert.PreviousAmount != 5999 || alert.Amount != 6999 || alert.AnnualImpact != 1000*12 || alert.Change != 0.167 {
		t.Errorf("unexpected alert: %+v", alert)
	}
}

func TestSmallPriceChangeIsNoise(t *testing.T) {
	b := newBuilder().monthly(12, "SAAS", func(i int) ledger.Cents {
		if i < 9 {
			return 10000
		}
		return 10300 // +3%, e.g. FX or tax
	})
	if a := Analyze(b.txs); len(a.Alerts) != 0 {
		t.Errorf("+3%% should not alert: %+v", a.Alerts)
	}
}

func TestUsageBasedBillHasNoPriceAlerts(t *testing.T) {
	amounts := []ledger.Cents{31200, 35800, 29900, 41000, 33300, 38800, 30100, 36600, 34900, 42000}
	a := Analyze(newBuilder().monthly(len(amounts), "AMAZON WEB SERVICES", func(i int) ledger.Cents { return amounts[i] }).txs)
	s := onlySeries(t, a)
	if s.Pricing != PricingVariable || len(a.Alerts) != 0 {
		t.Errorf("pricing=%s alerts=%+v", s.Pricing, a.Alerts)
	}
	// Annualised from the median of the last three bills (34900, 36600, 42000).
	if s.AnnualCost != 36600*12 {
		t.Errorf("annual cost = %v", s.AnnualCost)
	}
}

func TestCancelledSubscriptionIsInactive(t *testing.T) {
	b := newBuilder().monthly(6, "DROPBOX", price(1999)).add("2025-12-15", "STAPLES", 3000)
	a := Analyze(b.txs)
	s := onlySeries(t, a)
	if s.Status != StatusInactive || s.NextExpected != nil || s.AnnualCost != 0 {
		t.Errorf("unexpected series: %+v", s)
	}
	if a.Totals[0].InactiveCount != 1 || a.Totals[0].AnnualRecurring != 0 {
		t.Errorf("unexpected totals: %+v", a.Totals)
	}
}

func TestCurrenciesAreNeverMixed(t *testing.T) {
	b := newBuilder()
	for i := range 4 {
		d := b.start.AddMonths(i)
		b.addTx(d, "SPOTIFY", 1199, "USD")
		b.addTx(d, "SPOTIFY", 2190, "BRL")
	}
	a := Analyze(b.txs)
	if len(a.Series) != 2 || len(a.Totals) != 2 {
		t.Fatalf("want one series and one total per currency, got %d/%d", len(a.Series), len(a.Totals))
	}
	for _, total := range a.Totals {
		want := map[string]ledger.Cents{"USD": 1199 * 12, "BRL": 2190 * 12}[total.Currency]
		if total.AnnualRecurring != want {
			t.Errorf("%s annual = %v, want %v", total.Currency, total.AnnualRecurring, want)
		}
	}
}

func TestSeriesOrdering(t *testing.T) {
	b := newBuilder().
		monthly(12, "CHEAP", price(500)).
		monthly(12, "EXPENSIVE", price(9900))
	b.start = ledger.MustParseDate("2024-01-01")
	b.monthly(4, "OLD", price(1000)) // cancelled long ago
	a := Analyze(b.txs)
	got := []string{a.Series[0].Merchant, a.Series[1].Merchant, a.Series[2].Merchant}
	want := []string{"Expensive", "Cheap", "Old"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestEmptyInput(t *testing.T) {
	a := Analyze(nil)
	if a.Series == nil || a.Alerts == nil || a.Totals == nil {
		t.Error("slices should be empty, not nil, so the JSON has [] instead of null")
	}
}
