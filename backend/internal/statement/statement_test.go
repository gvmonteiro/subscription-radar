package statement

import (
	"errors"
	"strings"
	"testing"

	"subscription-radar/internal/ledger"
)

func TestParseAmount(t *testing.T) {
	cases := []struct {
		in   string
		want ledger.Cents
		ok   bool
	}{
		{"12.99", 1299, true},
		{"$1,234.56", 123456, true},
		{"R$ 1.234,56", 123456, true},
		{"1.234.567,89", 123456789, true},
		{"12,5", 1250, true},
		{"1,234", 123400, true}, // comma followed by 3 digits = thousands
		{"-45.00", -4500, true},
		{"$-45.00", -4500, true},
		{"(45.00)", -4500, true},
		{"45.00-", -4500, true},
		{"−7.10", -710, true}, // unicode minus
		{"USD 10", 1000, true},
		{"", 0, false},
		{"abc", 0, false},
		{"1-2", 0, false},
	}
	for _, c := range cases {
		got, ok := parseAmount(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("parseAmount(%q) = %v, %v; want %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestDetectDateOrder(t *testing.T) {
	cases := []struct {
		name      string
		dates     []string
		order     dateOrder
		ambiguous bool
	}{
		{"US", []string{"01/05/2026", "01/25/2026"}, monthFirst, false},
		{"rest of world", []string{"05/01/2026", "25/01/2026"}, dayFirst, false},
		{"ambiguous", []string{"01/05/2026", "02/06/2026"}, monthFirst, true},
		{"ISO only", []string{"2026-01-05"}, monthFirst, false},
	}
	for _, c := range cases {
		order, ambiguous := detectDateOrder(c.dates)
		if order != c.order || ambiguous != c.ambiguous {
			t.Errorf("%s: got %v/%v, want %v/%v", c.name, order, ambiguous, c.order, c.ambiguous)
		}
	}
}

func TestParseDate(t *testing.T) {
	cases := []struct {
		in    string
		order dateOrder
		want  string
	}{
		{"2026-03-01", monthFirst, "2026-03-01"},
		{"2026-03-01T10:22:00Z", monthFirst, "2026-03-01"},
		{"03/01/2026", monthFirst, "2026-03-01"},
		{"01/03/2026", dayFirst, "2026-03-01"},
		{"1.3.26", dayFirst, "2026-03-01"},
		{"Mar 1, 2026", monthFirst, "2026-03-01"},
		{"01-Mar-2026", monthFirst, "2026-03-01"},
	}
	for _, c := range cases {
		got, ok := parseDate(c.in, c.order)
		if !ok || got.String() != c.want {
			t.Errorf("parseDate(%q) = %v, %v; want %s", c.in, got, ok, c.want)
		}
	}
	if _, ok := parseDate("02/30/2026", monthFirst); ok {
		t.Error("February 30th should be rejected")
	}
}

func TestParseCardFormat(t *testing.T) {
	csv := "Date,Description,Amount\n" +
		"01/15/2026,NETFLIX.COM,15.49\n" +
		"01/20/2026,\"GITHUB, INC.\",84.00\n" +
		"01/25/2026,PAYMENT - THANK YOU,-500.00\n"
	res := mustParse(t, csv)
	if len(res.Transactions) != 2 || res.IgnoredCredits != 1 {
		t.Fatalf("got %d transactions, %d ignored", len(res.Transactions), res.IgnoredCredits)
	}
	got := res.Transactions[1]
	if got.Description != "GITHUB, INC." || got.Amount != 8400 || got.Currency != "USD" || got.Row != 3 {
		t.Errorf("unexpected transaction: %+v", got)
	}
}

func TestParseBankFormatPortuguese(t *testing.T) {
	// Semicolons, Portuguese headers, DD/MM dates, R$ with decimal comma,
	// spending as negative numbers and an Excel BOM.
	csv := "\xEF\xBB\xBFData;Descrição;Valor\n" +
		"15/01/2026;SPOTIFY;-R$ 21,90\n" +
		"20/01/2026;SALARIO;R$ 8.000,00\n" +
		"25/01/2026;ALUGUEL;-R$ 2.500,00\n"
	res := mustParse(t, csv)
	if len(res.Transactions) != 2 || res.IgnoredCredits != 1 {
		t.Fatalf("got %d transactions, %d ignored", len(res.Transactions), res.IgnoredCredits)
	}
	first := res.Transactions[0]
	if first.Date.String() != "2026-01-15" || first.Amount != 2190 || first.Currency != "BRL" {
		t.Errorf("unexpected transaction: %+v", first)
	}
	if !strings.Contains(strings.Join(res.Assumptions, " "), "negative") {
		t.Errorf("expected a bank-format assumption, got %v", res.Assumptions)
	}
}

func TestParseDebitCreditColumns(t *testing.T) {
	csv := "Posted Date,Payee,Debit,Credit,Currency\n" +
		"2026-01-03,ZOOM.US,14.99,,usd\n" +
		"2026-01-04,REFUND,,14.99,usd\n"
	res := mustParse(t, csv)
	if len(res.Transactions) != 1 || res.IgnoredCredits != 1 {
		t.Fatalf("got %d transactions, %d ignored", len(res.Transactions), res.IgnoredCredits)
	}
	if res.Transactions[0].Currency != "USD" {
		t.Errorf("currency column should be used and upper-cased, got %q", res.Transactions[0].Currency)
	}
}

func TestParseReportsBadRows(t *testing.T) {
	csv := "Date,Description,Amount\n" +
		"not a date,FIGMA,45.00\n" +
		"01/09/2026,FIGMA,forty five\n" +
		"01/10/2026,,45.00\n" +
		"01/11/2026,FIGMA,45.00\n"
	res := mustParse(t, csv)
	if len(res.Transactions) != 1 || len(res.Warnings) != 3 {
		t.Fatalf("got %d transactions, warnings %v", len(res.Transactions), res.Warnings)
	}
	if res.Warnings[0].Row != 2 {
		t.Errorf("warning should point at file line 2, got %d", res.Warnings[0].Row)
	}
}

func TestParseRejectsUnknownLayout(t *testing.T) {
	for name, input := range map[string]string{
		"missing columns": "foo,bar\n1,2\n",
		"header only":     "Date,Description,Amount\n",
		"empty":           "",
	} {
		_, err := Parse(strings.NewReader(input), Options{})
		var formatErr *FormatError
		if !errors.As(err, &formatErr) {
			t.Errorf("%s: want FormatError, got %v", name, err)
		}
	}
}

func mustParse(t *testing.T, csv string) *Result {
	t.Helper()
	res, err := Parse(strings.NewReader(csv), Options{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return res
}
