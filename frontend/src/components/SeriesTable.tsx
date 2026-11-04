import { Fragment, useState } from "react";
import type { Series } from "../api/types";
import { cadenceLabel, cadenceUnit, date, money } from "../format";

interface Props {
  series: Series[];
  focusId: string | null;
  onFocus: (id: string | null) => void;
}

export function SeriesTable({ series, focusId, onFocus }: Props) {
  const [expanded, setExpanded] = useState<string | null>(null);
  const maxAnnual = Math.max(...series.map((s) => s.annualCost), 1);

  return (
    <div className="table-scroll">
      <table className="series-table">
        <thead>
          <tr>
            <th>Merchant</th>
            <th>Billed</th>
            <th className="num">Price</th>
            <th className="num">Per year</th>
            <th className="wide-only">Last charge</th>
            <th>Next expected</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {series.map((s) => {
            const open = expanded === s.id;
            return (
              <Fragment key={s.id}>
                <tr
                  className={`${focusId === s.id ? "focused" : ""} ${s.status === "inactive" ? "inactive" : ""}`}
                  onMouseEnter={() => onFocus(s.id)}
                  onMouseLeave={() => onFocus(null)}
                >
                  <td>
                    <button className="expand" onClick={() => setExpanded(open ? null : s.id)} aria-expanded={open}>
                      <span className={`chevron${open ? " open" : ""}`} aria-hidden="true">
                        ›
                      </span>
                      <span className="merchant">{s.merchant}</span>
                    </button>
                    <span className="raw" title="As it appears on the statement">
                      {s.charges[s.charges.length - 1]?.description}
                    </span>
                  </td>
                  <td>{cadenceLabel[s.cadence]}</td>
                  <td className="num">
                    <Price series={s} />
                  </td>
                  <td className="num">
                    <div className="bar-cell">
                      <span>{s.status === "active" ? money(s.annualCost, s.currency, { whole: true }) : "—"}</span>
                      <span className="bar" style={{ width: `${(s.annualCost / maxAnnual) * 100}%` }} />
                    </div>
                  </td>
                  <td className="wide-only">{date(s.lastDate)}</td>
                  <td>{s.nextExpected ? date(s.nextExpected) : "—"}</td>
                  <td>
                    <StatusBadge series={s} />
                  </td>
                </tr>
                {open && (
                  <tr className="details">
                    <td colSpan={7} className="details-cell">
                      <Details series={s} />
                    </td>
                  </tr>
                )}
              </Fragment>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function Price({ series: s }: { series: Series }) {
  if (s.pricing === "variable") {
    return (
      <span title="Usage-based: the amount changes every bill">
        ~{money(s.typicalAmount, s.currency)} <span className="muted small">varies</span>
      </span>
    );
  }
  const previous = s.priceIncreasedOn ? s.charges.find((c) => c.date < s.priceIncreasedOn!)?.amount : undefined;
  return (
    <span>
      {money(s.latestAmount, s.currency)}
      <span className="muted small">/{cadenceUnit[s.cadence]}</span>
      {previous !== undefined && <span className="was">was {money(previous, s.currency)}</span>}
    </span>
  );
}

function StatusBadge({ series: s }: { series: Series }) {
  if (s.duplicates.length > 0) return <span className="badge danger">Billed twice</span>;
  if (s.status === "inactive") return <span className="badge neutral">Looks cancelled</span>;
  if (s.priceIncreasedOn) return <span className="badge warning">Price up</span>;
  return <span className="badge ok">Active</span>;
}

function Details({ series: s }: { series: Series }) {
  const all = [...s.charges.map((c) => ({ ...c, duplicate: false })), ...s.duplicates.map((d) => ({ ...d, duplicate: true }))].sort(
    (a, b) => b.date.localeCompare(a.date) || b.row - a.row,
  );
  return (
    <div className="details-body">
      <p className="muted">
        {s.charges.length} charges since {date(s.firstDate)} · {money(s.totalPaid, s.currency)} paid in this period ·{" "}
        {s.confidence === "high" ? "very regular schedule" : "mostly regular schedule"}
      </p>
      <ul className="charge-list">
        {all.map((c) => (
          <li key={c.row} className={c.duplicate ? "duplicate" : ""}>
            <span>{date(c.date)}</span>
            <span className="num">{money(c.amount, c.currency)}</span>
            <span className="muted small">row {c.row}</span>
            {c.duplicate && <span className="badge danger">duplicate</span>}
          </li>
        ))}
      </ul>
    </div>
  );
}
