// Package merchant turns raw statement descriptions into a stable merchant key.
//
// The same subscription shows up as "SLACK*T0A1B2C3", "SLACK*T9Z8Y7X6", ...
// every month, and processors wrap merchants ("PAYPAL *SPOTIFY", "SQ *BLUE
// BOTTLE"). Grouping by the raw text would never find a pattern, so we strip
// the noise with a short list of explainable rules instead of fuzzy matching.
package merchant

import (
	"regexp"
	"strings"
	"unicode"
)

// Merchant is the grouping key plus a human-friendly name.
type Merchant struct {
	Key  string
	Name string
}

// Payment processors put the real merchant after the "*".
var processors = set("PAYPAL", "PP", "SQ", "SQU", "TST", "PADDLE", "PADDLE.NET", "FS", "2CO", "STRIPE", "EBANX", "MP", "MERCADOPAGO", "PAGSEGURO", "PAG")

// Platforms sell many products, so the product after "*" is part of the name
// ("GOOGLE *WORKSPACE" and "GOOGLE *CLOUD" are different subscriptions).
var platforms = set("GOOGLE", "APPLE", "APL", "MICROSOFT", "MSFT")

// Well-known descriptors that the generic rules would mangle.
var rewrites = strings.NewReplacer("APPLE.COM/BILL", "APPLE", "AMAZON.COM", "AMAZON")

// Words banks prepend to every card line.
var leadingNoise = set("POS", "PURCHASE", "DEBIT", "CARD", "RECURRING", "CHECKCARD", "COMPRA", "PGTO", "PAG")

// Everything after a legal suffix is usually an address.
var corporateSuffixes = set("INC", "LLC", "LTD", "LIMITED", "CORP", "CORPORATION", "GMBH", "LTDA", "PLC", "PTY", "BV", "SA", "SAS")

// Trailing state and country codes: "SPOTIFY USA NY" -> "SPOTIFY".
var locationCodes = set(
	"US", "USA", "UK", "GB", "IE", "NL", "BR", "MX", "AR", "CL", "PE", "ES", "PT", "DE", "FR", "CAN",
	"AL", "AK", "AZ", "AR", "CA", "CO", "CT", "DE", "FL", "GA", "HI", "ID", "IL", "IN", "IA", "KS", "KY",
	"LA", "ME", "MD", "MA", "MI", "MN", "MS", "MO", "MT", "NE", "NV", "NH", "NJ", "NM", "NY", "NC", "ND",
	"OH", "OK", "OR", "PA", "RI", "SC", "SD", "TN", "TX", "UT", "VT", "VA", "WA", "WV", "WI", "WY", "DC",
	"SP", "RJ", "MG", "CDMX",
)

// Brand spellings that title-casing would get wrong.
var displayOverrides = map[string]string{
	"AMZN MKTP":           "Amazon Marketplace",
	"AMZN":                "Amazon",
	"AMAZON WEB SERVICES": "Amazon Web Services",
	"AWS":                 "AWS",
	"1PASSWORD":           "1Password",
	"RD STATION":          "RD Station",
	"GITHUB":              "GitHub",
	"HUBSPOT":             "HubSpot",
	"LINKEDIN":            "LinkedIn",
	"OPENAI":              "OpenAI",
	"YOUTUBE":             "YouTube",
	"WEWORK":              "WeWork",
}

var domainRe = regexp.MustCompile(`^[A-Z0-9-]+(\.[A-Z0-9-]+)+\.?$`)

// Normalize extracts the merchant from a raw description.
func Normalize(description string) Merchant {
	s := rewrites.Replace(strings.ToUpper(strings.TrimSpace(description)))

	// 1. "PREFIX*REST": decide which side names the merchant.
	if before, after, found := strings.Cut(s, "*"); found {
		prefix := strings.Join(dropLeadingNoise(strings.Fields(before)), " ")
		rest := strings.TrimSpace(after)
		switch {
		case prefix == "" || processors[prefix]:
			s = rest
		case platforms[prefix]:
			s = prefix + " " + rest
		default:
			s = prefix // what follows is an order or reference id
		}
	}

	// 2. Token clean-up.
	var tokens, domains []string
	for _, t := range dropLeadingNoise(strings.Fields(s)) {
		if digitCount(t) >= 2 {
			continue // reference ids, store numbers, phone numbers
		}
		if isDomain(t) {
			domains = append(domains, mainLabel(t))
			continue
		}
		t = strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return r
			}
			return -1
		}, t)
		if t == "" {
			continue
		}
		if corporateSuffixes[t] {
			break
		}
		tokens = append(tokens, t)
	}

	// 3. Trailing location codes, as long as something is left to name the merchant.
	for len(tokens) > 0 && locationCodes[tokens[len(tokens)-1]] && (len(tokens) > 1 || len(domains) > 0) {
		tokens = tokens[:len(tokens)-1]
	}

	// 4. Descriptions that are just a domain: "NETFLIX.COM" -> "NETFLIX".
	if len(tokens) == 0 && len(domains) > 0 {
		tokens = domains[:1]
	}
	if len(tokens) == 0 {
		tokens = strings.Fields(strings.ToUpper(description))
	}

	key := strings.Join(tokens, " ")
	return Merchant{Key: key, Name: displayName(key)}
}

func dropLeadingNoise(tokens []string) []string {
	for len(tokens) > 1 && leadingNoise[tokens[0]] {
		tokens = tokens[1:]
	}
	return tokens
}

func digitCount(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			n++
		}
	}
	return n
}

// isDomain accepts "NETFLIX.COM" or "HELP.UBER.COM" but not "INC." or "S.A.".
func isDomain(t string) bool {
	if !domainRe.MatchString(t) {
		return false
	}
	labels := strings.Split(strings.TrimSuffix(t, "."), ".")
	tld := labels[len(labels)-1]
	return len(tld) >= 2 && strings.IndexFunc(tld, func(r rune) bool { return r < 'A' || r > 'Z' }) < 0 && len(labels[0]) > 1
}

// mainLabel returns the registrable name: "HELP.UBER.COM" -> "UBER".
func mainLabel(domain string) string {
	labels := strings.Split(strings.TrimSuffix(domain, "."), ".")
	if labels[0] == "WWW" {
		labels = labels[1:]
	}
	if len(labels) >= 2 {
		return labels[len(labels)-2]
	}
	return labels[0]
}

func displayName(key string) string {
	if name, ok := displayOverrides[key]; ok {
		return name
	}
	words := strings.Fields(key)
	for i, w := range words {
		runes := []rune(strings.ToLower(w))
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, item := range items {
		m[item] = true
	}
	return m
}
