import { useState } from "react";
import type { AnalyzeResponse } from "../api/types";
import { date } from "../format";
import { downloadReport } from "../report";
import { AlertList } from "./AlertList";
import { ParseNotes } from "./ParseNotes";
import { SeriesTable } from "./SeriesTable";
import { Summary } from "./Summary";
import { Timeline } from "./Timeline";

export function Results({ result }: { result: AnalyzeResponse }) {
  const { analysis, parse } = result;
  // Hovering a row in the table or the timeline highlights it in both.
  const [focusId, setFocusId] = useState<string | null>(null);

  return (
    <div className="results">
      <div className="results-header">
        <div>
          <h2>Recurring spend</h2>
          <p className="muted">
            {date(analysis.periodStart)} – {date(analysis.referenceDate)} · {analysis.transactionCount} charges from{" "}
            {analysis.merchantCount} merchants
          </p>
        </div>
        {analysis.series.length > 0 && (
          <button className="button secondary" onClick={() => downloadReport(result)}>
            Download report (CSV)
          </button>
        )}
      </div>

      {analysis.totals.map((totals) => (
        <Summary key={totals.currency} totals={totals} multiCurrency={analysis.totals.length > 1} />
      ))}

      {analysis.series.length === 0 ? (
        <div className="empty">
          <h3>No recurring charges found</h3>
          <p className="muted">
            We need at least three on-schedule charges for monthly plans (two for yearly ones). Try a longer statement period.
          </p>
        </div>
      ) : (
        <>
          <AlertList alerts={analysis.alerts} onFocus={setFocusId} />
          <section className="panel">
            <div className="panel-header">
              <h3>Timeline</h3>
              <Legend />
            </div>
            <Timeline analysis={analysis} focusId={focusId} onFocus={setFocusId} />
          </section>
          <section className="panel">
            <div className="panel-header">
              <h3>All recurring charges</h3>
              <span className="muted">Sorted by yearly cost</span>
            </div>
            <SeriesTable series={analysis.series} focusId={focusId} onFocus={setFocusId} />
          </section>
        </>
      )}

      <ParseNotes parse={parse} />
    </div>
  );
}

function Legend() {
  return (
    <ul className="legend" aria-label="Legend">
      <li>
        <span className="dot charge" /> Charge
      </li>
      <li>
        <span className="dot increased" /> New price
      </li>
      <li>
        <span className="dot duplicate" /> Duplicate
      </li>
      <li>
        <span className="dot projected" /> Next expected
      </li>
    </ul>
  );
}
