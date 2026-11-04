import { useRef, useState, type DragEvent } from "react";
import { SAMPLE_URL } from "../api/client";

interface Props {
  loading: boolean;
  error: string | null;
  onFile: (file: File) => void;
  onSample: () => void;
}

export function UploadScreen({ loading, error, onFile, onSample }: Props) {
  const input = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);

  function handleDrop(event: DragEvent) {
    event.preventDefault();
    setDragging(false);
    const file = event.dataTransfer.files[0];
    if (file && !loading) onFile(file);
  }

  return (
    <section className="upload">
      <div className="upload-copy">
        <p className="eyebrow">For finance teams and founders</p>
        <h1>Find every subscription hiding on your company card.</h1>
        <p className="lede">
          Upload a card or bank statement export (CSV). In seconds you see what renews, what it costs per year,
          what got more expensive and what was billed twice.
        </p>
      </div>

      <div
        className={`dropzone${dragging ? " dragging" : ""}${loading ? " busy" : ""}`}
        onDragOver={(e) => {
          e.preventDefault();
          setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={handleDrop}
        onClick={() => !loading && input.current?.click()}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => (e.key === "Enter" || e.key === " ") && input.current?.click()}
        aria-label="Upload a CSV statement"
      >
        <input
          ref={input}
          type="file"
          accept=".csv,text/csv"
          hidden
          onChange={(e) => {
            const file = e.target.files?.[0];
            if (file) onFile(file);
            e.target.value = "";
          }}
        />
        {loading ? (
          <>
            <div className="spinner" aria-hidden="true" />
            <strong>Analysing your statement…</strong>
          </>
        ) : (
          <>
            <UploadIcon />
            <strong>Drop a CSV here or click to choose a file</strong>
            <span className="muted">Any bank or card export with date, description and amount columns</span>
          </>
        )}
      </div>

      {error && (
        <div className="error-box" role="alert">
          {error}
        </div>
      )}

      <div className="upload-actions">
        <button className="button primary" onClick={onSample} disabled={loading}>
          Try the demo statement
        </button>
        <a className="link" href={SAMPLE_URL} download>
          Download the demo CSV
        </a>
      </div>

      <ul className="facts">
        <li>
          <strong>No AI, no guesswork.</strong> Every result comes from a rule you can read: cadence, amount and timing.
        </li>
        <li>
          <strong>Nothing is stored.</strong> The file is analysed in memory and discarded.
        </li>
        <li>
          <strong>Works with real exports.</strong> Comma or semicolon, US or day-first dates, $1,234.56 or 1.234,56.
        </li>
      </ul>
    </section>
  );
}

function UploadIcon() {
  return (
    <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
      <path d="M12 16V4m0 0-4 4m4-4 4 4" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M4 15v3a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-3" strokeLinecap="round" />
    </svg>
  );
}
