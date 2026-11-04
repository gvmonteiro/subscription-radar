package sample_test

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"testing"

	"subscription-radar/internal/recurring"
	"subscription-radar/internal/sample"
	"subscription-radar/internal/statement"
)

// TestEndToEndAcrossSeeds runs the full pipeline (CSV -> parser -> detector)
// on many randomly generated statements and requires the exact answer key:
// every planted subscription found (recall) and nothing else flagged
// (precision), including alerts.
func TestEndToEndAcrossSeeds(t *testing.T) {
	for seed := uint64(1); seed <= 50; seed++ {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			s := sample.Generate(seed)
			parsed, err := statement.Parse(bytes.NewReader(s.CSV), statement.Options{})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if len(parsed.Warnings) > 0 {
				t.Fatalf("unexpected warnings: %v", parsed.Warnings)
			}
			analysis := recurring.Analyze(parsed.Transactions)

			var got, want []string
			for _, series := range analysis.Series {
				got = append(got, fmt.Sprintf("%s|%s|%s", series.Merchant, series.Cadence, series.Status))
			}
			for _, e := range s.Series {
				want = append(want, fmt.Sprintf("%s|%s|%s", e.Merchant, e.Cadence, e.Status))
			}
			assertSameSet(t, "series", got, want)

			got, want = nil, nil
			for _, a := range analysis.Alerts {
				got = append(got, fmt.Sprintf("%s|%s|%s", a.Kind, a.Merchant, a.Date))
			}
			for _, e := range s.Alerts {
				want = append(want, fmt.Sprintf("%s|%s|%s", e.Kind, e.Merchant, e.Date))
			}
			assertSameSet(t, "alerts", got, want)
		})
	}
}

// TestExampleFiles guards the CSVs in examples/ so the README stays true.
func TestExampleFiles(t *testing.T) {
	cases := []struct {
		file   string
		series []string
		alerts []string
	}{
		{
			file:   "simple-statement.csv",
			series: []string{"Netflix|monthly|active", "Google Workspace|monthly|active", "Spotify|monthly|active"},
		},
		{
			file: "subscriptions-with-alerts.csv",
			series: []string{
				"Adobe|monthly|active", "GitHub|monthly|active", "Zoom|monthly|active",
				"Netflix|monthly|active", "Spotify|monthly|active", "Dropbox|monthly|inactive",
			},
			alerts: []string{
				"duplicate|Spotify|2026-05-09", "duplicate|Zoom|2026-03-06",
				"price_increase|GitHub|2026-05-15", "price_increase|Adobe|2026-04-12",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			f, err := os.Open("../../../examples/" + c.file)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			parsed, err := statement.Parse(f, statement.Options{})
			if err != nil {
				t.Fatal(err)
			}
			analysis := recurring.Analyze(parsed.Transactions)

			var got []string
			for _, s := range analysis.Series {
				got = append(got, fmt.Sprintf("%s|%s|%s", s.Merchant, s.Cadence, s.Status))
			}
			assertSameSet(t, "series", got, c.series)
			got = nil
			for _, a := range analysis.Alerts {
				got = append(got, fmt.Sprintf("%s|%s|%s", a.Kind, a.Merchant, a.Date))
			}
			assertSameSet(t, "alerts", got, c.alerts)
		})
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	a, b := sample.Generate(sample.DefaultSeed), sample.Generate(sample.DefaultSeed)
	if !bytes.Equal(a.CSV, b.CSV) {
		t.Fatal("same seed produced different CSVs")
	}
}

func assertSameSet(t *testing.T, label string, got, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)
	missing, extra := diff(want, got), diff(got, want)
	if len(missing) > 0 || len(extra) > 0 {
		t.Errorf("%s mismatch\n  missing (false negatives): %v\n  extra (false positives): %v", label, missing, extra)
	}
}

func diff(a, b []string) []string {
	inB := map[string]bool{}
	for _, x := range b {
		inB[x] = true
	}
	var out []string
	for _, x := range a {
		if !inB[x] {
			out = append(out, x)
		}
	}
	return out
}
