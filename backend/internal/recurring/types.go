package recurring

import "subscription-radar/internal/ledger"

type Cadence string

const (
	Weekly    Cadence = "weekly"
	Biweekly  Cadence = "biweekly"
	Monthly   Cadence = "monthly"
	Quarterly Cadence = "quarterly"
	Yearly    Cadence = "yearly"
)

type cadenceSpec struct {
	cadence Cadence
	// days is the expected gap between charges.
	days float64
	// tolerance is how early or late a charge may post and still be on
	// schedule. Monthly needs room for 28-31 day months plus weekend posting.
	tolerance float64
	perYear   int64
	// minCount is how many on-schedule charges we need before calling it
	// recurring. Two monthly charges can be a coincidence; three rarely are.
	minCount int
	// months > 0 projects the next charge on the same calendar day
	// (billing renews on the 15th, not every 30.44 days); 0 uses days.
	months int
}

var cadences = []cadenceSpec{
	{Weekly, 7, 1, 52, 4, 0},
	{Biweekly, 14, 2, 26, 3, 0},
	{Monthly, 30.44, 4, 12, 3, 1},
	{Quarterly, 91.31, 8, 4, 3, 3},
	{Yearly, 365.25, 12, 1, 2, 12},
}

func (c cadenceSpec) next(last ledger.Date) ledger.Date {
	if c.months > 0 {
		return last.AddMonths(c.months)
	}
	return last.AddDays(int(c.days))
}

const (
	StatusActive   = "active"
	StatusInactive = "inactive"

	PricingFixed    = "fixed"
	PricingVariable = "variable"

	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
)

// Series is one detected subscription or recurring bill.
type Series struct {
	ID          string  `json:"id"`
	Merchant    string  `json:"merchant"`
	MerchantKey string  `json:"merchantKey"`
	Currency    string  `json:"currency"`
	Cadence     Cadence `json:"cadence"`
	// Pricing is "fixed" (same price each time; price changes allowed) or
	// "variable" (usage-based, e.g. cloud bills).
	Pricing    string `json:"pricing"`
	Confidence string `json:"confidence"`
	Status     string `json:"status"`
	// Charges are the on-schedule charges, oldest first.
	Charges []ledger.Transaction `json:"charges"`
	// Duplicates are extra charges that look like double billing.
	Duplicates []ledger.Transaction `json:"duplicates"`
	// PriceIncreasedOn is the first charge at the current, higher price.
	PriceIncreasedOn *ledger.Date `json:"priceIncreasedOn"`
	TypicalAmount    ledger.Cents `json:"typicalAmount"`
	LatestAmount     ledger.Cents `json:"latestAmount"`
	FirstDate        ledger.Date  `json:"firstDate"`
	LastDate         ledger.Date  `json:"lastDate"`
	// NextExpected is null when the series looks cancelled.
	NextExpected *ledger.Date `json:"nextExpected"`
	// AnnualCost projects the next 12 months; 0 when inactive.
	AnnualCost ledger.Cents `json:"annualCost"`
	// TotalPaid is everything paid in the statement period, duplicates included.
	TotalPaid ledger.Cents `json:"totalPaid"`
}

type AlertKind string

const (
	AlertDuplicate     AlertKind = "duplicate"
	AlertPriceIncrease AlertKind = "price_increase"
)

// Alert is something the user should act on.
type Alert struct {
	Kind     AlertKind `json:"kind"`
	SeriesID string    `json:"seriesId"`
	Merchant string    `json:"merchant"`
	Currency string    `json:"currency"`
	// Date is the duplicate charge's date, or the first charge at the new price.
	Date ledger.Date `json:"date"`
	// Amount is the duplicate charge (refundable) or the new price.
	Amount ledger.Cents `json:"amount"`
	Row    int          `json:"row"`

	// Duplicate only: the legitimate charge it duplicates.
	OriginalDate *ledger.Date `json:"originalDate,omitempty"`

	// Price increase only.
	PreviousAmount ledger.Cents `json:"previousAmount,omitempty"`
	Change         float64      `json:"change,omitempty"`
	AnnualImpact   ledger.Cents `json:"annualImpact,omitempty"`
}

// Totals are per currency: amounts in different currencies are never summed.
type Totals struct {
	Currency         string       `json:"currency"`
	AnnualRecurring  ledger.Cents `json:"annualRecurring"`
	MonthlyRecurring ledger.Cents `json:"monthlyRecurring"`
	ActiveCount      int          `json:"activeCount"`
	InactiveCount    int          `json:"inactiveCount"`
	// Refundable is the sum of duplicate charges.
	Refundable ledger.Cents `json:"refundable"`
	// PriceIncreaseImpact is the extra yearly cost from price increases.
	PriceIncreaseImpact ledger.Cents `json:"priceIncreaseImpact"`
}

type Analysis struct {
	PeriodStart ledger.Date `json:"periodStart"`
	// ReferenceDate is the last date in the statement. It stands in for
	// "today" so the same file always gives the same answer.
	ReferenceDate    ledger.Date `json:"referenceDate"`
	TransactionCount int         `json:"transactionCount"`
	MerchantCount    int         `json:"merchantCount"`
	Series           []Series    `json:"series"`
	Alerts           []Alert     `json:"alerts"`
	Totals           []Totals    `json:"totals"`
}
