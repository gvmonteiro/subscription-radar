import type { Alert } from "../api/types";
import { date, money, percent } from "../format";

interface Props {
  alerts: Alert[];
  onFocus: (seriesId: string | null) => void;
}

export function AlertList({ alerts, onFocus }: Props) {
  if (alerts.length === 0) {
    return (
      <div className="all-clear">
        <CheckIcon /> No duplicate charges or price increases found.
      </div>
    );
  }
  return (
    <section className="alerts" aria-label="Needs attention">
      <h3>Needs attention</h3>
      <ul>
        {alerts.map((alert) => (
          <li
            key={`${alert.kind}-${alert.seriesId}-${alert.date}`}
            className={`alert ${alert.kind === "duplicate" ? "danger" : "warning"}`}
            onMouseEnter={() => onFocus(alert.seriesId)}
            onMouseLeave={() => onFocus(null)}
          >
            {alert.kind === "duplicate" ? (
              <>
                <div className="alert-title">
                  {alert.merchant} charged you twice
                  <span className="alert-amount">{money(alert.amount, alert.currency)}</span>
                </div>
                <p>
                  Charged on {date(alert.originalDate)} and again on {date(alert.date)}. Ask the vendor or your card
                  issuer for a refund of the second charge.
                </p>
              </>
            ) : (
              <>
                <div className="alert-title">
                  {alert.merchant} went up {percent(alert.change)}
                  <span className="alert-amount">+{money(alert.annualImpact, alert.currency)}/yr</span>
                </div>
                <p>
                  {money(alert.previousAmount, alert.currency)} → {money(alert.amount, alert.currency)} since{" "}
                  {date(alert.date)}. Check that the new price (or the extra seats) was expected.
                </p>
              </>
            )}
            <span className="alert-row">Row {alert.row} in your file</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function CheckIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" aria-hidden="true">
      <path d="m5 12 5 5L20 7" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
