import { useEffect, useState } from "react";

type Props = {
  /** When the data on screen was collected; null before the first load. */
  updatedAt: number | string | null;
  isRefreshing: boolean;
  /** The last refresh failed; the data on screen is from before it. */
  failed?: boolean;
};

/**
 * How fresh the data on screen is: "Updated 2 min ago". Data refreshes in the
 * background on the connection's interval; the exact time is in the tooltip.
 */
export function DataFreshness({ updatedAt, isRefreshing, failed = false }: Props) {
  const now = useNow(30_000);
  const time = updatedAt === null ? null : new Date(updatedAt);
  const state = isRefreshing ? "refreshing" : failed ? "failed" : time ? "fresh" : "waiting";
  // A background refresh only turns the dot blue: with short intervals a
  // changing label would flicker every few seconds.
  const text = time ? `Updated ${relativeTime(time.getTime(), now)}` : isRefreshing ? "Loading…" : "No data yet";
  const title = [
    time ? `Collected at ${time.toLocaleString()}` : null,
    isRefreshing && time ? "Refreshing in the background…" : null,
    failed ? "The last refresh failed; showing the previous data." : null,
  ].filter(Boolean).join("\n");

  return (
    <div className={`data-freshness ${state}`}>
      <span className="data-freshness-status" title={title || undefined} aria-live="polite">
        <i aria-hidden="true" />{text}
      </span>
    </div>
  );
}

function useNow(intervalMs: number) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(id);
  }, [intervalMs]);
  return now;
}

function relativeTime(at: number, now: number) {
  const seconds = Math.max(0, Math.round((now - at) / 1000));
  if (seconds < 45) return "just now";
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} h ago`;
  return new Date(at).toLocaleDateString();
}
