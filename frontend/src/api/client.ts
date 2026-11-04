import type { AnalyzeResponse } from "./types";

export class ApiError extends Error {}

export async function analyzeFile(file: File): Promise<AnalyzeResponse> {
  const form = new FormData();
  form.append("file", file);
  return request("/api/analyze", { method: "POST", body: form });
}

/** Loads the demo statement from the backend and analyses it like any upload. */
export async function analyzeSample(): Promise<AnalyzeResponse> {
  const response = await fetch(SAMPLE_URL);
  if (!response.ok) throw new ApiError("Could not load the sample statement. Is the API running?");
  const csv = await response.blob();
  return analyzeFile(new File([csv], "sample-statement.csv", { type: "text/csv" }));
}

export const SAMPLE_URL = "/api/sample";

async function request<T>(url: string, init: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(url, init);
  } catch {
    throw new ApiError("Could not reach the API. Start it with `go run ./cmd/server` in backend/.");
  }
  const body = await response.json().catch(() => null);
  if (!response.ok) {
    throw new ApiError(body?.error ?? `The server answered ${response.status}.`);
  }
  return body as T;
}
