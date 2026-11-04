# Subscription Radar: write-up

## The problem

Forgotten subscriptions are a problem people have in their personal lives, and companies have the same problem on their corporate cards. You forget to cancel a tool nobody uses anymore, or you start a free trial and forget it was going to convert into a paid plan. Every company card statement hides this layer of recurring spend: SaaS seats, forgotten trials, and annual renewals that nobody remembers until they hit. Finance teams at small companies usually find these by hand, line by line, if they find them at all. The expensive cases are the quiet ones:

- a vendor that **raised its price**;
- a service that **billed twice**;
- a tool that is **still renewing** after the team stopped using it.

Small amounts also feel harmless. "It's only $10" becomes $120 by the end of the year, and more if the price goes up. Showing the yearly cost is what changes the decision.

I picked this problem for three reasons:

- It is real, and close to a corporate card product: spend visibility is the first thing a finance lead asks for.
- It is small enough to solve properly in three days.
- Its value is easy to see. You drop a file and get a number, such as "$14,674 a year in subscriptions, $14.99 billed twice, +$330 a year in price increases".

## Approach

I wanted to keep it simple and clear. The app is a Go API plus a React UI. You upload a CSV and get back three things:

- a yearly cost summary;
- alerts for duplicates and price increases;
- a timeline of every charge, plus a table you can expand down to the original statement rows.

The core is a three-stage pipeline:

1. **Parse any export.** The parser detects delimiter, header language, date order, number format and sign convention, and reports every assumption it made.
2. **Normalise merchants.** Rules turn `SLACK*T0A1B2C3` into `Slack`.
3. **Detect recurrence.** Each merchant's charges are checked for cadence, regularity, amount stability, duplicates, price changes and status. A subscription is a pattern: the same merchant name, a similar amount, and a regular gap between charges. The median of those gaps decides whether a charge is weekly, monthly, quarterly or yearly. The median is used so that one late payment doesn't break the detection.

## Key decisions and trade-offs

- **Rules, not ML or an LLM.** A finance user has to trust and act on the result ("ask Zoom for a refund"), so every flag must be explainable in one sentence. Rules are also deterministic, fast and free to run. The cost is coverage: unusual descriptors need a new rule rather than "just working".
- **Precision over recall.** I would rather miss a subscription than send someone chasing a false one. That is why a monthly plan needs three charges, why price alerts require a *stable* old price (so cloud bills never trigger them), and why duplicates are only checked inside recurring series (so two coffees in one day are not "double billing").
- **The statement's last date is "today".** The same file always gives the same answer, which makes tests reproducible and the output easy to reason about. The trade-off is that "looks cancelled" is relative to the file, not to the calendar.
- **Integer cents and UTC calendar dates.** No float drift in totals and no daylight-saving off-by-one errors in day gaps. Monthly renewals are projected on the same calendar day, not every 30.44 days.
- **Go standard library only, and a stateless API.** I didn't want anything too elaborate at this stage. There is nothing to install or configure, nothing is stored, and one binary also serves the UI.
- **A synthetic statement with an answer key.** I could not use real statements, so I wrote a seeded generator. It plants 11 subscriptions plus about 380 realistic noise charges, some deliberately designed to look recurring. The end-to-end test runs 50 seeds and requires exact precision and recall. This turned tuning the thresholds from guesswork into a measurable loop.

## How I used AI tools

<!-- Edit this section to reflect how you actually worked. -->
I built this with Claude Code as a pair programmer.

- **My part:** I chose the problem and the domain framing, asked for a Go backend, set the precision-over-recall stance, and reviewed the detection rules and thresholds.
- **AI's part:** scaffolding, the first versions of the parser and detector, the tests and the SVG timeline. I leaned on AI the most for the frontend, where the goal was simplicity and clarity rather than custom engineering.
- **Testing in a real browser:** I ran the app in a real browser (desktop, mobile width, light and dark themes) and fixed what came up. The fixes included a layout overflow, overlapping chart labels, and a renewal-date bug that the tests caught (+30 days instead of "same day next month").

The tests and the answer-key generator were what made fast AI-assisted iteration safe.

## What I would do next

1. **Act on the findings:** export alerts to Slack or email, and add a "cancel or keep" decision per subscription with an owner.
2. **Connect to live card data** instead of CSV uploads, and attribute each subscription to the cardholder or team.
3. **AI-assisted recognition.** Use AI to recognise merchants and suggest aliases learned from user corrections (`AMZN` = `AMAZON`). The rules would still make the final decision, so recall improves without losing explainability.
4. **Seat-level insight:** for per-seat tools such as Slack and GitHub, flag growth and compare it against headcount.
5. **Multi-currency totals** with FX conversion for companies that spend in several currencies.
