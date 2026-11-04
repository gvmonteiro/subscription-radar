import type { Cadence } from "./api/types";

const moneyFormatters = new Map<string, Intl.NumberFormat>();

export function money(amount: number, currency: string, opts: { whole?: boolean } = {}): string {
  const key = `${currency}|${opts.whole ? 0 : 2}`;
  let formatter = moneyFormatters.get(key);
  if (!formatter) {
    try {
      formatter = new Intl.NumberFormat("en-US", {
        style: "currency",
        currency,
        minimumFractionDigits: opts.whole ? 0 : 2,
        maximumFractionDigits: opts.whole ? 0 : 2,
      });
    } catch {
      // Unknown currency code in the file: fall back to a plain number.
      formatter = new Intl.NumberFormat("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    }
    moneyFormatters.set(key, formatter);
  }
  return formatter.format(amount);
}

const dateFormatter = new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric", year: "numeric", timeZone: "UTC" });
const shortDateFormatter = new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric", timeZone: "UTC" });

/** ISO date -> "Aug 15, 2026". Parsed as UTC so the day never shifts. */
export function date(iso: string): string {
  return dateFormatter.format(new Date(`${iso}T00:00:00Z`));
}

export function shortDate(iso: string): string {
  return shortDateFormatter.format(new Date(`${iso}T00:00:00Z`));
}

export function percent(fraction: number): string {
  return `${Math.round(fraction * 100)}%`;
}

export const cadenceLabel: Record<Cadence, string> = {
  weekly: "Weekly",
  biweekly: "Every 2 weeks",
  monthly: "Monthly",
  quarterly: "Quarterly",
  yearly: "Yearly",
};

export const cadenceUnit: Record<Cadence, string> = {
  weekly: "week",
  biweekly: "2 weeks",
  monthly: "month",
  quarterly: "quarter",
  yearly: "year",
};

export function toEpochDay(iso: string): number {
  return Date.parse(`${iso}T00:00:00Z`) / 86_400_000;
}
