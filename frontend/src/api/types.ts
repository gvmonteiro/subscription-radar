// Mirrors the JSON produced by backend/internal/api and backend/internal/recurring.
// Money is a number in major units (12.99); dates are "YYYY-MM-DD".

export interface Transaction {
  row: number;
  date: string;
  description: string;
  amount: number;
  currency: string;
}

export type Cadence = "weekly" | "biweekly" | "monthly" | "quarterly" | "yearly";

export interface Series {
  id: string;
  merchant: string;
  merchantKey: string;
  currency: string;
  cadence: Cadence;
  pricing: "fixed" | "variable";
  confidence: "high" | "medium";
  status: "active" | "inactive";
  charges: Transaction[];
  duplicates: Transaction[];
  priceIncreasedOn: string | null;
  typicalAmount: number;
  latestAmount: number;
  firstDate: string;
  lastDate: string;
  nextExpected: string | null;
  annualCost: number;
  totalPaid: number;
}

interface AlertBase {
  seriesId: string;
  merchant: string;
  currency: string;
  date: string;
  amount: number;
  row: number;
}

export interface DuplicateAlert extends AlertBase {
  kind: "duplicate";
  originalDate: string;
}

export interface PriceIncreaseAlert extends AlertBase {
  kind: "price_increase";
  previousAmount: number;
  change: number;
  annualImpact: number;
}

export type Alert = DuplicateAlert | PriceIncreaseAlert;

export interface Totals {
  currency: string;
  annualRecurring: number;
  monthlyRecurring: number;
  activeCount: number;
  inactiveCount: number;
  refundable: number;
  priceIncreaseImpact: number;
}

export interface Analysis {
  periodStart: string;
  referenceDate: string;
  transactionCount: number;
  merchantCount: number;
  series: Series[];
  alerts: Alert[];
  totals: Totals[];
}

export interface ParseSummary {
  transactionCount: number;
  ignoredCredits: number;
  warnings: { row: number; message: string }[];
  assumptions: string[];
}

export interface AnalyzeResponse {
  fileName: string;
  parse: ParseSummary;
  analysis: Analysis;
}
