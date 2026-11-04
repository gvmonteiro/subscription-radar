import type { Analysis, Series } from "../api/types";
import { cadenceLabel, date, money, shortDate, toEpochDay } from "../format";

interface Props {
  analysis: Analysis;
  focusId: string | null;
  onFocus: (id: string | null) => void;
}

// Fixed drawing coordinates; the SVG scales to the container width.
const WIDTH = 1000;
const LABEL_WIDTH = 200;
const RIGHT_PAD = 20;
const HEADER = 48;
const ROW = 40;
/** Days shown after the statement ends, so next month's charges are visible. */
const LOOKAHEAD_DAYS = 40;

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

export function Timeline({ analysis, focusId, onFocus }: Props) {
  const start = new Date(`${analysis.periodStart}T00:00:00Z`);
  const startDay = toEpochDay(`${start.getUTCFullYear()}-${String(start.getUTCMonth() + 1).padStart(2, "0")}-01`);
  const refDay = toEpochDay(analysis.referenceDate);
  const endDay = refDay + LOOKAHEAD_DAYS;
  const x = (iso: string) => LABEL_WIDTH + ((toEpochDay(iso) - startDay) / (endDay - startDay)) * (WIDTH - LABEL_WIDTH - RIGHT_PAD);
  const height = HEADER + analysis.series.length * ROW + 8;

  // Month grid lines.
  const ticks: { x: number; label: string }[] = [];
  const cursor = new Date(startDay * 86_400_000);
  while (toEpochDay(cursor.toISOString().slice(0, 10)) <= endDay) {
    const month = cursor.getUTCMonth();
    const showYear = month === 0 || ticks.length === 0;
    ticks.push({
      x: x(cursor.toISOString().slice(0, 10)),
      label: showYear ? `${MONTHS[month]} '${String(cursor.getUTCFullYear()).slice(2)}` : MONTHS[month]!,
    });
    cursor.setUTCMonth(month + 1);
  }
  const refX = x(analysis.referenceDate);

  return (
    <div className="timeline-scroll">
      <svg className="timeline" viewBox={`0 0 ${WIDTH} ${height}`} role="img" aria-label="Timeline of recurring charges">
        {ticks.map((t) => (
          <g key={t.x}>
            <line x1={t.x} x2={t.x} y1={HEADER - 22} y2={height} className="grid" />
            <text x={t.x + 4} y={HEADER - 28} className="tick">
              {t.label}
            </text>
          </g>
        ))}

        {analysis.series.map((s, i) => (
          <Row key={s.id} series={s} y={HEADER + i * ROW} x={x} endDay={endDay} focused={focusId === s.id} dimmed={focusId !== null && focusId !== s.id} onFocus={onFocus} />
        ))}

        <line x1={refX} x2={refX} y1={HEADER - 14} y2={height} className="ref-line" />
        <text x={refX - 4} y={HEADER - 8} className="ref-label" textAnchor="end">
          Statement end · {shortDate(analysis.referenceDate)}
        </text>
      </svg>
    </div>
  );
}

interface RowProps {
  series: Series;
  y: number;
  x: (iso: string) => number;
  endDay: number;
  focused: boolean;
  dimmed: boolean;
  onFocus: (id: string | null) => void;
}

function Row({ series: s, y, x, endDay, focused, dimmed, onFocus }: RowProps) {
  const mid = y + ROW / 2;
  const inactive = s.status === "inactive";
  const increasedFrom = s.priceIncreasedOn ? toEpochDay(s.priceIncreasedOn) : Infinity;
  const showNext = s.nextExpected !== null && toEpochDay(s.nextExpected) <= endDay;
  const subtitle = inactive
    ? `Stopped after ${shortDate(s.lastDate)}`
    : `${cadenceLabel[s.cadence]} · ${money(s.annualCost, s.currency, { whole: true })}/yr`;

  const classes = ["row", inactive && "inactive", focused && "focused", dimmed && "dimmed"].filter(Boolean).join(" ");

  return (
    <g className={classes} onMouseEnter={() => onFocus(s.id)} onMouseLeave={() => onFocus(null)}>
      <rect x={0} y={y} width={WIDTH} height={ROW} className="row-bg" rx={6} />
      <text x={12} y={mid - 3} className="row-title">
        {truncate(s.merchant, 24)}
      </text>
      <text x={12} y={mid + 12} className="row-sub">
        {subtitle}
      </text>

      <line x1={x(s.firstDate)} x2={showNext ? x(s.nextExpected!) : x(s.lastDate)} y1={mid} y2={mid} className="track" />

      {s.charges.map((c) => {
        const increased = toEpochDay(c.date) >= increasedFrom;
        return (
          <circle key={c.row} cx={x(c.date)} cy={mid} r={4.5} className={increased ? "dot-increased" : "dot-charge"}>
            <title>
              {date(c.date)} · {money(c.amount, c.currency)}
              {increased ? " · new price" : ""}
            </title>
          </circle>
        );
      })}

      {s.duplicates.map((d) => (
        <g key={d.row}>
          <circle cx={x(d.date)} cy={mid} r={9} className="dot-duplicate-ring" />
          <circle cx={x(d.date)} cy={mid} r={4.5} className="dot-duplicate">
            <title>
              {date(d.date)} · {money(d.amount, d.currency)} · charged twice
            </title>
          </circle>
        </g>
      ))}

      {showNext && (
        <circle cx={x(s.nextExpected!)} cy={mid} r={4.5} className="dot-projected">
          <title>
            Next expected {date(s.nextExpected!)} · ~{money(s.pricing === "fixed" ? s.latestAmount : s.typicalAmount, s.currency)}
          </title>
        </circle>
      )}
    </g>
  );
}

function truncate(text: string, max: number): string {
  return text.length > max ? `${text.slice(0, max - 1)}…` : text;
}
