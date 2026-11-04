// Package recurring finds subscriptions and recurring bills in a list of
// charges, and flags duplicate billing and price increases.
//
// The detector is deliberately rule-based: every result can be explained with
// a sentence ("charged every ~30 days, 12 times, same price"), which matters
// when a finance team has to act on it.
package recurring

import (
	"cmp"
	"math"
	"slices"

	"subscription-radar/internal/ledger"
	"subscription-radar/internal/merchant"
)

// Tuning knobs. Each one is a product decision documented in the README.
const (
	// Two charges of the same amount this close together are double billing,
	// not a weekly plan (weekly gaps are at least 6 days).
	duplicateWindowDays      = 3
	duplicateAmountTolerance = 0.01

	// Share of gaps that must match the cadence. 0.75 survives one missed or
	// late month in a year of data.
	minRegularity = 0.75

	// Relative spread (median absolute deviation / median) of amounts.
	fixedPriceSpread  = 0.02 // at most 2%: a fixed-price plan
	maxVariableSpread = 0.25 // above 25%: regular visits, not a bill (e.g. weekly groceries)

	// A price change smaller than this is rounding, FX or tax noise.
	minPriceIncrease   = 0.05
	samePriceTolerance = 0.01

	// No charge for 1.5 cycles (plus tolerance) means it was probably cancelled.
	inactiveAfterCycles = 1.5

	// High confidence needs almost perfectly regular gaps.
	highConfidenceRegularity = 0.9
)

// Analyze groups charges by merchant and currency and returns every recurring
// series it can find. Transactions do not need to be sorted.
func Analyze(txs []ledger.Transaction) Analysis {
	a := Analysis{
		TransactionCount: len(txs),
		Series:           []Series{},
		Alerts:           []Alert{},
		Totals:           []Totals{},
	}
	if len(txs) == 0 {
		return a
	}

	sorted := slices.Clone(txs)
	slices.SortStableFunc(sorted, func(x, y ledger.Transaction) int {
		return cmp.Or(x.Date.Compare(y.Date), cmp.Compare(x.Row, y.Row))
	})
	a.PeriodStart = sorted[0].Date
	a.ReferenceDate = sorted[len(sorted)-1].Date

	type group struct {
		merchant merchant.Merchant
		currency string
		txs      []ledger.Transaction
	}
	groups := map[string]*group{}
	var order []string
	merchants := map[string]bool{}
	for _, t := range sorted {
		m := merchant.Normalize(t.Description)
		merchants[m.Key] = true
		id := m.Key + "|" + t.Currency
		g, ok := groups[id]
		if !ok {
			g = &group{merchant: m, currency: t.Currency}
			groups[id] = g
			order = append(order, id)
		}
		g.txs = append(g.txs, t)
	}
	a.MerchantCount = len(merchants)

	for _, id := range order {
		g := groups[id]
		series, alerts, ok := detect(id, g.merchant, g.currency, g.txs, a.ReferenceDate)
		if ok {
			a.Series = append(a.Series, series)
			a.Alerts = append(a.Alerts, alerts...)
		}
	}

	sortSeries(a.Series)
	sortAlerts(a.Alerts)
	a.Totals = totals(a.Series, a.Alerts)
	return a
}

type duplicatePair struct{ original, duplicate ledger.Transaction }

// detect decides whether one merchant's charges form a recurring series.
func detect(id string, m merchant.Merchant, currency string, txs []ledger.Transaction, ref ledger.Date) (Series, []Alert, bool) {
	charges, duplicates := splitDuplicates(txs)
	if len(charges) < 2 {
		return Series{}, nil, false
	}

	// 1. Cadence: the median gap is robust to one late or missed charge.
	gaps := make([]float64, len(charges)-1)
	for i := 1; i < len(charges); i++ {
		gaps[i-1] = float64(charges[i-1].Date.DaysUntil(charges[i].Date))
	}
	spec, ok := matchCadence(median(gaps))
	if !ok || len(charges) < spec.minCount {
		return Series{}, nil, false
	}

	// 2. Regularity: most gaps, not just the median, must fit the cadence.
	onSchedule := 0
	for _, gap := range gaps {
		if math.Abs(gap-spec.days) <= spec.tolerance {
			onSchedule++
		}
	}
	regularity := float64(onSchedule) / float64(len(gaps))
	if regularity < minRegularity {
		return Series{}, nil, false
	}

	// 3. Amounts: fixed price, usage-based, or too erratic to be a bill.
	amounts := make([]ledger.Cents, len(charges))
	for i, c := range charges {
		amounts[i] = c.Amount
	}
	spread := relativeSpread(amounts)
	increase, hasIncrease := detectPriceIncrease(amounts)
	pricing := PricingVariable
	if hasIncrease || spread <= fixedPriceSpread {
		pricing = PricingFixed
	}
	if pricing == PricingVariable && spread > maxVariableSpread {
		return Series{}, nil, false
	}

	// 4. Status and projection.
	last := charges[len(charges)-1]
	s := Series{
		ID:            id,
		Merchant:      m.Name,
		MerchantKey:   m.Key,
		Currency:      currency,
		Cadence:       spec.cadence,
		Pricing:       pricing,
		Confidence:    ConfidenceMedium,
		Status:        StatusInactive,
		Charges:       charges,
		Duplicates:    []ledger.Transaction{},
		TypicalAmount: ledger.Cents(math.Round(median(amounts))),
		LatestAmount:  last.Amount,
		FirstDate:     charges[0].Date,
		LastDate:      last.Date,
	}
	if regularity >= highConfidenceRegularity && len(charges) > spec.minCount {
		s.Confidence = ConfidenceHigh
	}
	for _, c := range charges {
		s.TotalPaid += c.Amount
	}

	daysSinceLast := float64(last.Date.DaysUntil(ref))
	if daysSinceLast <= spec.days*inactiveAfterCycles+spec.tolerance {
		s.Status = StatusActive
		next := spec.next(last.Date)
		s.NextExpected = &next
		estimate := last.Amount
		if pricing == PricingVariable {
			// Usage-based: the recent median is a better guess than one bill.
			estimate = ledger.Cents(math.Round(median(amounts[max(0, len(amounts)-3):])))
		}
		s.AnnualCost = estimate * ledger.Cents(spec.perYear)
	}

	// 5. Alerts.
	var alerts []Alert
	for _, d := range duplicates {
		s.Duplicates = append(s.Duplicates, d.duplicate)
		s.TotalPaid += d.duplicate.Amount
		originalDate := d.original.Date
		alerts = append(alerts, Alert{
			Kind:         AlertDuplicate,
			SeriesID:     id,
			Merchant:     m.Name,
			Currency:     currency,
			Date:         d.duplicate.Date,
			Amount:       d.duplicate.Amount,
			Row:          d.duplicate.Row,
			OriginalDate: &originalDate,
		})
	}
	if hasIncrease {
		since := charges[increase.index].Date
		s.PriceIncreasedOn = &since
		if s.Status == StatusActive {
			alerts = append(alerts, Alert{
				Kind:           AlertPriceIncrease,
				SeriesID:       id,
				Merchant:       m.Name,
				Currency:       currency,
				Date:           since,
				Amount:         increase.to,
				Row:            charges[increase.index].Row,
				PreviousAmount: increase.from,
				Change:         math.Round((float64(increase.to)/float64(increase.from)-1)*1000) / 1000,
				AnnualImpact:   (increase.to - increase.from) * ledger.Cents(spec.perYear),
			})
		}
	}
	return s, alerts, true
}

// splitDuplicates separates double billing from the regular series: a charge
// within a few days of the previous one, for the same amount.
func splitDuplicates(txs []ledger.Transaction) ([]ledger.Transaction, []duplicatePair) {
	var charges []ledger.Transaction
	var duplicates []duplicatePair
	for _, t := range txs {
		if n := len(charges); n > 0 {
			prev := charges[n-1]
			if prev.Date.DaysUntil(t.Date) <= duplicateWindowDays && closeTo(t.Amount, prev.Amount, duplicateAmountTolerance) {
				duplicates = append(duplicates, duplicatePair{original: prev, duplicate: t})
				continue
			}
		}
		charges = append(charges, t)
	}
	return charges, duplicates
}

type priceIncrease struct {
	index    int // first charge at the new price
	from, to ledger.Cents
}

// detectPriceIncrease looks for a stable old price followed by a run of
// charges at a higher current price. Usage-based bills never qualify because
// their old "price" is not stable.
func detectPriceIncrease(amounts []ledger.Cents) (priceIncrease, bool) {
	latest := amounts[len(amounts)-1]
	start := len(amounts) - 1
	for start > 0 && closeTo(amounts[start-1], latest, samePriceTolerance) {
		start--
	}
	if start == 0 {
		return priceIncrease{}, false
	}
	before := amounts[:start]
	if relativeSpread(before) > fixedPriceSpread {
		return priceIncrease{}, false
	}
	from := ledger.Cents(math.Round(median(before)))
	if from <= 0 || float64(latest)/float64(from)-1 < minPriceIncrease {
		return priceIncrease{}, false
	}
	return priceIncrease{index: start, from: from, to: latest}, true
}

func matchCadence(medianGap float64) (cadenceSpec, bool) {
	for _, spec := range cadences {
		if math.Abs(medianGap-spec.days) <= spec.tolerance {
			return spec, true
		}
	}
	return cadenceSpec{}, false
}

func sortSeries(series []Series) {
	slices.SortStableFunc(series, func(x, y Series) int {
		if x.Status != y.Status {
			if x.Status == StatusActive {
				return -1
			}
			return 1
		}
		if x.Status == StatusActive {
			return cmp.Or(cmp.Compare(y.AnnualCost, x.AnnualCost), cmp.Compare(x.Merchant, y.Merchant))
		}
		return y.LastDate.Compare(x.LastDate)
	})
}

func sortAlerts(alerts []Alert) {
	rank := map[AlertKind]int{AlertDuplicate: 0, AlertPriceIncrease: 1}
	slices.SortStableFunc(alerts, func(x, y Alert) int {
		return cmp.Or(cmp.Compare(rank[x.Kind], rank[y.Kind]), y.Date.Compare(x.Date))
	})
}

func totals(series []Series, alerts []Alert) []Totals {
	byCurrency := map[string]*Totals{}
	var order []string
	get := func(currency string) *Totals {
		t, ok := byCurrency[currency]
		if !ok {
			t = &Totals{Currency: currency}
			byCurrency[currency] = t
			order = append(order, currency)
		}
		return t
	}
	for _, s := range series {
		t := get(s.Currency)
		if s.Status == StatusActive {
			t.ActiveCount++
			t.AnnualRecurring += s.AnnualCost
		} else {
			t.InactiveCount++
		}
	}
	for _, a := range alerts {
		t := get(a.Currency)
		switch a.Kind {
		case AlertDuplicate:
			t.Refundable += a.Amount
		case AlertPriceIncrease:
			t.PriceIncreaseImpact += a.AnnualImpact
		}
	}
	out := make([]Totals, 0, len(order))
	for _, c := range order {
		t := byCurrency[c]
		t.MonthlyRecurring = ledger.Cents(math.Round(float64(t.AnnualRecurring) / 12))
		out = append(out, *t)
	}
	slices.SortStableFunc(out, func(x, y Totals) int { return cmp.Compare(y.AnnualRecurring, x.AnnualRecurring) })
	return out
}

// ---------------------------------------------------------------------------
// Small statistics helpers
// ---------------------------------------------------------------------------

func median[T ~int64 | ~float64](values []T) float64 {
	if len(values) == 0 {
		return 0
	}
	s := slices.Clone(values)
	slices.Sort(s)
	mid := len(s) / 2
	if len(s)%2 == 1 {
		return float64(s[mid])
	}
	return (float64(s[mid-1]) + float64(s[mid])) / 2
}

// relativeSpread is the median absolute deviation divided by the median:
// like a coefficient of variation, but one outlier cannot dominate it.
func relativeSpread(amounts []ledger.Cents) float64 {
	m := median(amounts)
	if m == 0 {
		return 0
	}
	deviations := make([]float64, len(amounts))
	for i, a := range amounts {
		deviations[i] = math.Abs(float64(a) - m)
	}
	return median(deviations) / m
}

func closeTo(a, b ledger.Cents, tolerance float64) bool {
	reference := math.Max(math.Abs(float64(a)), math.Abs(float64(b)))
	return math.Abs(float64(a-b)) <= tolerance*reference
}
