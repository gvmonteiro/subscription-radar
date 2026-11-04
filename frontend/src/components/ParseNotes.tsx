import type { ParseSummary } from "../api/types";

const MAX_WARNINGS = 8;

/** Explains how the file was read, so no number is a black box. */
export function ParseNotes({ parse }: { parse: ParseSummary }) {
  return (
    <details className="parse-notes">
      <summary>How we read your file</summary>
      <ul>
        <li>
          {parse.transactionCount} charges analysed, {parse.ignoredCredits} payments or refunds ignored.
        </li>
        {parse.assumptions.map((a) => (
          <li key={a}>{a}</li>
        ))}
        {parse.warnings.length > 0 && (
          <li>
            {parse.warnings.length} row{parse.warnings.length === 1 ? " was" : "s were"} skipped:
            <ul>
              {parse.warnings.slice(0, MAX_WARNINGS).map((w) => (
                <li key={w.row}>
                  Row {w.row}: {w.message}
                </li>
              ))}
              {parse.warnings.length > MAX_WARNINGS && <li>…and {parse.warnings.length - MAX_WARNINGS} more</li>}
            </ul>
          </li>
        )}
        <li>
          A subscription needs 3 on-schedule charges (2 for yearly plans). Charges are grouped by merchant after removing
          reference numbers, processor prefixes and locations.
        </li>
      </ul>
    </details>
  );
}
