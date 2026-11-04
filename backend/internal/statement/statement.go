// Package statement turns a bank or card CSV export into normalised charges.
//
// Exports differ in delimiter, header names and language, date order, number
// format and sign convention. The parser detects each of those from the data
// and reports every assumption it made, so the user can see why a number is
// what it is.
package statement

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"subscription-radar/internal/ledger"
)

// FormatError means the input is not a usable statement. Its message is safe
// to show to the user.
type FormatError struct{ msg string }

func (e *FormatError) Error() string { return e.msg }

func formatErrorf(format string, args ...any) error {
	return &FormatError{msg: fmt.Sprintf(format, args...)}
}

// Warning is a row that was skipped because it could not be read.
type Warning struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// Result is the parsed statement plus everything the user should know about
// how it was read.
type Result struct {
	Transactions []ledger.Transaction `json:"-"`
	Warnings     []Warning            `json:"warnings"`
	// IgnoredCredits counts money-in rows (card payments, refunds, income).
	IgnoredCredits int      `json:"ignoredCredits"`
	Assumptions    []string `json:"assumptions"`
}

type Options struct {
	// DefaultCurrency is used when there is no currency column or symbol.
	DefaultCurrency string
}

// Header names seen in real exports (English, Portuguese, Spanish), compared
// after lower-casing and stripping accents and punctuation.
var columnAliases = map[string][]string{
	"date":        {"date", "transaction date", "trans date", "posted date", "posting date", "data", "fecha"},
	"description": {"description", "merchant", "name", "payee", "details", "memo", "descricao", "historico", "estabelecimento", "lancamento", "descripcion", "concepto"},
	"amount":      {"amount", "value", "valor", "importe", "monto"},
	"debit":       {"debit", "withdrawal", "debito", "cargo", "saida"},
	"currency":    {"currency", "moeda", "moneda", "ccy"},
}

type row struct {
	line   int
	fields []string
}

func (r row) get(col int) string {
	if col < 0 || col >= len(r.fields) {
		return ""
	}
	return r.fields[col]
}

// Parse reads a CSV statement. It returns a *FormatError when the file cannot
// be interpreted at all; individual bad rows become warnings instead.
func Parse(input io.Reader, opts Options) (*Result, error) {
	if opts.DefaultCurrency == "" {
		opts.DefaultCurrency = "USD"
	}
	header, rows, err := readRows(input)
	if err != nil {
		return nil, err
	}

	col := map[string]int{}
	for name, aliases := range columnAliases {
		col[name] = findColumn(header, aliases)
	}
	var missing []string
	if col["date"] < 0 {
		missing = append(missing, "date")
	}
	if col["description"] < 0 {
		missing = append(missing, "description")
	}
	if col["amount"] < 0 && col["debit"] < 0 {
		missing = append(missing, "amount")
	}
	if len(missing) > 0 {
		list := strings.Join(missing, ", ")
		if n := len(missing); n > 1 {
			list = strings.Join(missing[:n-1], ", ") + " and " + missing[n-1]
		}
		return nil, formatErrorf("Could not find the %s column(s). Expected a header row such as \"Date, Description, Amount\".", list)
	}

	res := &Result{Warnings: []Warning{}, Assumptions: []string{}}

	rawDates := make([]string, len(rows))
	for i, r := range rows {
		rawDates[i] = r.get(col["date"])
	}
	order, ambiguous := detectDateOrder(rawDates)
	if ambiguous {
		res.Assumptions = append(res.Assumptions, "Every date fits both MM/DD and DD/MM; assumed MM/DD/YYYY.")
	}

	// With one signed "amount" column, banks show spending as negative and card
	// issuers show it as positive. The majority sign tells us which.
	amountCol := col["amount"]
	useDebitCol := amountCol < 0
	spendSign := ledger.Cents(1)
	if !useDebitCol {
		var negatives, positives int
		for _, r := range rows {
			if v, ok := parseAmount(r.get(amountCol)); ok && v != 0 {
				if v < 0 {
					negatives++
				} else {
					positives++
				}
			}
		}
		if negatives > positives {
			spendSign = -1
			res.Assumptions = append(res.Assumptions, "Charges are negative amounts (bank format); positive rows were treated as income or refunds and ignored.")
		} else {
			res.Assumptions = append(res.Assumptions, "Charges are positive amounts (card format); negative rows were treated as payments or refunds and ignored.")
		}
	} else {
		amountCol = col["debit"]
		res.Assumptions = append(res.Assumptions, "Separate debit and credit columns; only debits were analysed.")
	}

	for _, r := range rows {
		rawDate, rawAmount := r.get(col["date"]), r.get(amountCol)
		description := strings.Join(strings.Fields(r.get(col["description"])), " ")

		date, ok := parseDate(rawDate, order)
		if !ok {
			res.Warnings = append(res.Warnings, Warning{r.line, fmt.Sprintf("Unreadable date %q", rawDate)})
			continue
		}

		var spent ledger.Cents
		if useDebitCol {
			v, ok := parseAmount(rawAmount)
			if ok && v < 0 {
				v = -v
			}
			spent = v // empty debit cell = credit row = 0
		} else {
			v, ok := parseAmount(rawAmount)
			if !ok {
				res.Warnings = append(res.Warnings, Warning{r.line, fmt.Sprintf("Unreadable amount %q", rawAmount)})
				continue
			}
			spent = v * spendSign
		}
		if spent <= 0 {
			res.IgnoredCredits++
			continue
		}
		if description == "" {
			res.Warnings = append(res.Warnings, Warning{r.line, "Empty description"})
			continue
		}

		currency := strings.ToUpper(strings.TrimSpace(r.get(col["currency"])))
		if currency == "" {
			currency = currencyFromSymbol(rawAmount)
		}
		if currency == "" {
			currency = opts.DefaultCurrency
		}

		res.Transactions = append(res.Transactions, ledger.Transaction{
			Row:         r.line,
			Date:        date,
			Description: description,
			Amount:      spent,
			Currency:    currency,
		})
	}
	return res, nil
}

// readRows reads the header (normalised) and the non-empty data rows.
func readRows(input io.Reader) ([]string, []row, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, nil, err
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM from Excel exports

	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = detectDelimiter(data)
	reader.FieldsPerRecord = -1 // some exports have trailing empty columns
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	var header []string
	var rows []row
	for {
		fields, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, formatErrorf("The file is not valid CSV: %v", err)
		}
		if isBlank(fields) {
			continue
		}
		line, _ := reader.FieldPos(0)
		if header == nil {
			header = make([]string, len(fields))
			for i, f := range fields {
				header[i] = normalizeHeader(f)
			}
			continue
		}
		rows = append(rows, row{line: line, fields: fields})
	}
	if header == nil || len(rows) == 0 {
		return nil, nil, formatErrorf("The file has no data rows.")
	}
	return header, rows, nil
}

// detectDelimiter picks the most frequent of comma, semicolon and tab on the
// first line. Semicolons are common in Latin American and European exports.
func detectDelimiter(data []byte) rune {
	firstLine, _, _ := bytes.Cut(data, []byte("\n"))
	best, bestCount := ',', 0
	for _, candidate := range []rune{',', ';', '\t'} {
		count, inQuotes := 0, false
		for _, c := range string(firstLine) {
			if c == '"' {
				inQuotes = !inQuotes
			} else if c == candidate && !inQuotes {
				count++
			}
		}
		if count > bestCount {
			best, bestCount = candidate, count
		}
	}
	return best
}

func isBlank(fields []string) bool {
	for _, f := range fields {
		if strings.TrimSpace(f) != "" {
			return false
		}
	}
	return true
}

var accentReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

func normalizeHeader(h string) string {
	h = accentReplacer.Replace(strings.ToLower(h))
	h = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r
		}
		return ' '
	}, h)
	return strings.Join(strings.Fields(h), " ")
}

// findColumn prefers an exact header match, then a prefix match such as
// "amount usd" for "Amount (USD)".
func findColumn(header []string, aliases []string) int {
	if i := slices.IndexFunc(header, func(h string) bool { return slices.Contains(aliases, h) }); i >= 0 {
		return i
	}
	return slices.IndexFunc(header, func(h string) bool {
		return slices.ContainsFunc(aliases, func(a string) bool { return strings.HasPrefix(h, a+" ") })
	})
}
