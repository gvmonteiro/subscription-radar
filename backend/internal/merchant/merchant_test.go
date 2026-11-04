package merchant

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		in, key, name string
	}{
		// Reference ids after "*" change every month.
		{"SLACK*T0A1B2C3", "SLACK", "Slack"},
		{"DROPBOX*7Q2K9X", "DROPBOX", "Dropbox"},
		{"AMZN MKTP US*2K4AB19Z0", "AMZN MKTP", "Amazon Marketplace"},
		// Processors wrap the real merchant.
		{"PAYPAL *SPOTIFY", "SPOTIFY", "Spotify"},
		{"SQ *BLUE BOTTLE COFFEE", "BLUE BOTTLE COFFEE", "Blue Bottle Coffee"},
		{"TST* JOE'S PIZZA", "JOES PIZZA", "Joes Pizza"},
		{"PAG*ESCOLADEINGLES", "ESCOLADEINGLES", "Escoladeingles"},
		{"MP*MERCADOLIVRE", "MERCADOLIVRE", "Mercadolivre"},
		// Platforms keep the product name.
		{"GOOGLE *WORKSPACE", "GOOGLE WORKSPACE", "Google Workspace"},
		{"GOOGLE *CLOUD", "GOOGLE CLOUD", "Google Cloud"},
		{"APPLE.COM/BILL", "APPLE", "Apple"},
		// Domains, phone numbers, legal suffixes and locations.
		{"NETFLIX.COM", "NETFLIX", "Netflix"},
		{"ZOOM.US 888-799-9666 CA", "ZOOM", "Zoom"},
		{"UBER *TRIP HELP.UBER.COM CA", "UBER", "Uber"},
		{"GITHUB, INC. GITHUB.COM CA", "GITHUB", "GitHub"},
		{"FIGMA INC SAN FRANCISCO CA", "FIGMA", "Figma"},
		{"AMAZON WEB SERVICES AWS.AMAZON.COM WA", "AMAZON WEB SERVICES", "Amazon Web Services"},
		{"SPOTIFY USA 877-778-1161 NY", "SPOTIFY", "Spotify"},
		{"STARBUCKS STORE #10442", "STARBUCKS STORE", "Starbucks Store"},
		// Bank prefixes and single-digit brand names survive.
		{"POS PURCHASE 1PASSWORD", "1PASSWORD", "1Password"},
		{"recurring netflix.com", "NETFLIX", "Netflix"},
		// Accented merchants (Latin America).
		{"CAFÉ DO PONTO SP", "CAFÉ DO PONTO", "Café Do Ponto"},
		// Nothing to strip.
		{"CA", "CA", "Ca"},
	}
	for _, c := range cases {
		got := Normalize(c.in)
		if got.Key != c.key || got.Name != c.name {
			t.Errorf("Normalize(%q) = {%q, %q}; want {%q, %q}", c.in, got.Key, got.Name, c.key, c.name)
		}
	}
}

func TestSameMerchantDifferentReferencesGroupTogether(t *testing.T) {
	a, b := Normalize("ADOBE *CREATIVE CLOUD 4029357733"), Normalize("ADOBE *ACROPRO SUBS")
	if a.Key != b.Key {
		t.Errorf("expected the same key, got %q and %q", a.Key, b.Key)
	}
}
