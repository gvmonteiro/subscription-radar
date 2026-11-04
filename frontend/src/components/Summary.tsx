import type { Totals } from "../api/types";
import { money } from "../format";

export function Summary({ totals, multiCurrency }: { totals: Totals; multiCurrency: boolean }) {
  const c = totals.currency;
  return (
    <div className="summary">
      {multiCurrency && <div className="summary-currency">{c}</div>}
      <Card label="Recurring spend per year" value={money(totals.annualRecurring, c, { whole: true })} hint={`${money(totals.monthlyRecurring, c)} per month`} tone="primary" />
      <Card
        label="Active subscriptions"
        value={String(totals.activeCount)}
        hint={totals.inactiveCount > 0 ? `${totals.inactiveCount} more look cancelled` : "None look cancelled"}
      />
      <Card
        label="Billed twice"
        value={money(totals.refundable, c)}
        hint={totals.refundable > 0 ? "Likely refundable" : "No duplicate charges"}
        tone={totals.refundable > 0 ? "danger" : undefined}
      />
      <Card
        label="Price increases"
        value={totals.priceIncreaseImpact > 0 ? `+${money(totals.priceIncreaseImpact, c, { whole: true })}` : money(0, c, { whole: true })}
        hint={totals.priceIncreaseImpact > 0 ? "Extra cost per year" : "No price changes"}
        tone={totals.priceIncreaseImpact > 0 ? "warning" : undefined}
      />
    </div>
  );
}

function Card({ label, value, hint, tone }: { label: string; value: string; hint: string; tone?: "primary" | "danger" | "warning" }) {
  return (
    <div className={`card${tone ? ` ${tone}` : ""}`}>
      <span className="card-label">{label}</span>
      <span className="card-value">{value}</span>
      <span className="card-hint">{hint}</span>
    </div>
  );
}
