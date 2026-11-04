import { useState } from "react";
import { analyzeFile, analyzeSample, ApiError } from "./api/client";
import type { AnalyzeResponse } from "./api/types";
import { UploadScreen } from "./components/UploadScreen";
import { Results } from "./components/Results";

type State =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "done"; result: AnalyzeResponse };

export function App() {
  const [state, setState] = useState<State>({ status: "idle" });

  async function run(load: () => Promise<AnalyzeResponse>) {
    setState({ status: "loading" });
    try {
      setState({ status: "done", result: await load() });
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Something went wrong while analysing the file.";
      setState({ status: "error", message });
    }
  }

  return (
    <div className="app">
      <header className="topbar">
        <button className="brand" onClick={() => setState({ status: "idle" })} aria-label="Subscription Radar home">
          <RadarIcon />
          <span>Subscription Radar</span>
        </button>
        {state.status === "done" && (
          <div className="topbar-file">
            <span className="file-chip" title={state.result.fileName}>{state.result.fileName || "statement.csv"}</span>
            <button className="button secondary" onClick={() => setState({ status: "idle" })}>
              Analyze another file
            </button>
          </div>
        )}
      </header>

      <main>
        {state.status === "done" ? (
          <Results result={state.result} />
        ) : (
          <UploadScreen
            loading={state.status === "loading"}
            error={state.status === "error" ? state.message : null}
            onFile={(file) => run(() => analyzeFile(file))}
            onSample={() => run(analyzeSample)}
          />
        )}
      </main>
    </div>
  );
}

function RadarIcon() {
  return (
    <svg width="22" height="22" viewBox="0 0 32 32" aria-hidden="true">
      <circle cx="16" cy="16" r="13" fill="none" stroke="currentColor" strokeWidth="2.5" />
      <circle cx="16" cy="16" r="7" fill="none" stroke="currentColor" strokeWidth="2" opacity="0.5" />
      <path d="M16 16 L26 8" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
      <circle cx="16" cy="16" r="2.5" fill="currentColor" />
    </svg>
  );
}
