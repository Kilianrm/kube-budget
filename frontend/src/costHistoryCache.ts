import { useEffect, useState } from "react";
import { getCostForecast, getCostTrend, CostForecast, CostTrend } from "./backend";

// Spend history and forecasts are read from local captures. They are cached
// per request so a section shows the last answer at once and refreshes it in
// the background (stale-while-revalidate), and the Cost report provider warms
// the defaults as soon as a report arrives.

/** The windows the trend and "What changed?" panels offer; 7 days is the default. */
export const historyWindows = [
  { days: 1, label: "24h", bucket: "hour", previous: "previous 24 hours" },
  { days: 7, label: "7d", bucket: "day", previous: "previous 7 days" },
  { days: 30, label: "30d", bucket: "day", previous: "previous 30 days" },
] as const;
export const defaultHistoryWindow = 1;

type Entry<T> = { value?: T; request?: Promise<T>; version: string };
const cache = new Map<string, Entry<unknown>>();

/**
 * Loads key's value unless the cached one is from the same version; concurrent
 * callers share the request. version changes whenever the data behind it may
 * have changed, such as a new capture.
 */
function load<T>(key: string, version: string, fetch: () => Promise<T>): Promise<T> {
  const entry = cache.get(key) as Entry<T> | undefined;
  if (entry?.version === version) {
    if (entry.request) return entry.request;
    if (entry.value !== undefined) return Promise.resolve(entry.value);
  }
  const next: Entry<T> = { value: entry?.value, version };
  next.request = fetch()
    .then((value) => {
      next.value = value;
      return value;
    })
    .finally(() => {
      next.request = undefined;
    });
  // A failed request is retried by the next caller.
  next.request.catch(() => {
    if (cache.get(key) === next && next.value === undefined) cache.delete(key);
  });
  cache.set(key, next as Entry<unknown>);
  return next.request;
}

function useCached<T>(key: string, version: string, fetch: () => Promise<T>) {
  const [value, setValue] = useState<T | null>(() => ((cache.get(key) as Entry<T> | undefined)?.value ?? null));
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    const cached = (cache.get(key) as Entry<T> | undefined)?.value;
    setValue(cached ?? null);
    setError("");
    load(key, version, fetch)
      .then((next) => { if (!cancelled) setValue(next); })
      .catch((reason) => { if (!cancelled) setError(reason instanceof Error ? reason.message : String(reason)); });
    return () => { cancelled = true; };
    // fetch is derived from key.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, version]);

  return { value, error };
}

const trendKey = (clusterId: string, days: number, bucket: string) => `trend:${clusterId}:${days}:${bucket}`;
const forecastKey = (clusterId: string, runRateHourly: number) => `forecast:${clusterId}:${runRateHourly}`;

/** The spend trend for a window; version is the time of the latest capture. */
export function useCostTrend(clusterId: string, days: number, bucket: "hour" | "day", version: string) {
  const { value, error } = useCached(trendKey(clusterId, days, bucket), version, () => getCostTrend(clusterId, days, bucket));
  return { trend: value, error };
}

/** The month forecast at a run rate; version changes with captures and budget edits. */
export function useCostForecast(clusterId: string, runRateHourly: number, version: string) {
  const { value, error } = useCached(forecastKey(clusterId, runRateHourly), version, () => getCostForecast(clusterId, runRateHourly));
  return { forecast: value, error };
}

/** Loads what the Cost section opens on, so it is ready before it is visited. */
export function prefetchCostHistory(clusterId: string, runRateHourly: number, version: string) {
  const window = historyWindows[defaultHistoryWindow];
  const ignore = () => undefined;
  load(trendKey(clusterId, window.days, window.bucket), version, () => getCostTrend(clusterId, window.days, window.bucket)).catch(ignore);
  load(forecastKey(clusterId, runRateHourly), version, () => getCostForecast(clusterId, runRateHourly)).catch(ignore);
}

/**
 * Marks cached forecasts stale after something they depend on, like the
 * budget, changed. They still show until the next read replaces them.
 */
export function invalidateCostForecasts(clusterId: string) {
  for (const [key, entry] of cache) {
    if (key.startsWith(`forecast:${clusterId}:`)) entry.version = "";
  }
}

export type { CostForecast, CostTrend };
