import type { AnalyzeResponse } from "./api/types";

/** Builds a spreadsheet-friendly report of the findings and downloads it. */
export function downloadReport({ analysis, fileName }: AnalyzeResponse) {
  const header = ["Merchant", "Cadence", "Status", "Currency", "Current price", "Projected per year", "Last charge", "Next expected", "Charges", "Paid in period", "Flags"];
  const rows = analysis.series.map((s) => {
    const flags = [
      s.duplicates.length > 0 ? `billed twice (${s.duplicates.length})` : "",
      s.priceIncreasedOn ? `price up since ${s.priceIncreasedOn}` : "",
      s.pricing === "variable" ? "usage-based" : "",
    ].filter(Boolean);
    return [
      s.merchant,
      s.cadence,
      s.status,
      s.currency,
      (s.pricing === "fixed" ? s.latestAmount : s.typicalAmount).toFixed(2),
      s.annualCost.toFixed(2),
      s.lastDate,
      s.nextExpected ?? "",
      String(s.charges.length),
      s.totalPaid.toFixed(2),
      flags.join("; "),
    ];
  });

  const csv = [header, ...rows].map((r) => r.map(escape).join(",")).join("\n");
  const url = URL.createObjectURL(new Blob([csv], { type: "text/csv" }));
  const link = document.createElement("a");
  link.href = url;
  link.download = `subscriptions-${(fileName || "statement").replace(/\.csv$/i, "")}.csv`;
  link.click();
  URL.revokeObjectURL(url);
}

function escape(value: string): string {
  return /[",\n]/.test(value) ? `"${value.replace(/"/g, '""')}"` : value;
}
