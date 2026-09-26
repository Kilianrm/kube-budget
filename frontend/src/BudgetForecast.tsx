import { FormEvent, useState } from "react";
import { AlertCircle, CalendarClock, LoaderCircle, TrendingDown, TrendingUp, Wallet } from "lucide-react";
import { setCostBudget, CostDriver, CostForecast, CostReport } from "./backend";
import { defaultHistoryWindow, historyWindows, invalidateCostForecasts, useCostForecast, useCostTrend } from "./costHistoryCache";
import { formatMoney, hoursPerMonth } from "./costFormat";
import { Segmented } from "./Segmented";

const recordedColor = "#2a78d6";
const increaseColor = "#eb6834";
const decreaseColor = "#2a78d6";

export function BudgetForecastView({ report, clusterId }: { report: CostReport; clusterId: string }) {
  const [reload, setReload] = useState(0);
  const runRate = report.totals?.provisioned?.hourly ?? 0;
  const currency = report.currency;
  const { forecast, error } = useCostForecast(clusterId, runRate, reload ? `${report.generatedAt}:${reload}` : String(report.generatedAt));

  if (error) return <div className="cluster-screen-content"><div className="error-message"><AlertCircle size={16} /><span>{error}</span></div></div>;
  if (!forecast) return <div className="cluster-screen-content"><section className="cluster-panel"><div className="panel-footnote"><LoaderCircle className="spin" size={15} /> Building the month forecast...</div></section></div>;

  const month = forecast.forecast;
  const budget = forecast.budget;
  const spent = month.recordedUSD + month.estimatedUSD;
  const monthName = new Date(month.monthStart).toLocaleString(undefined, { month: "long" });
  const daysLeft = Math.ceil(month.remainingHours / 24);
  const uncapturedHours = Math.max(month.elapsedHours - month.coveredHours, 0);
  const budgetTone = !budget ? "neutral" : budget.state === "on-track" ? "good" : "warning";

  return (
    <div className="cluster-screen-content">
      <div className="cluster-metrics">
        <div className="cluster-metric neutral">
          <span>Spent this month</span>
          <strong>{formatMoney(spent, currency)}</strong>
          <small>{formatMoney(month.recordedUSD, currency)} recorded{month.estimatedUSD > 0 ? ` · ${formatMoney(month.estimatedUSD, currency)} estimated for ${Math.round(uncapturedHours)} uncaptured h` : ""}</small>
        </div>
        <div className="cluster-metric neutral">
          <span>Forecast for {monthName}</span>
          <strong>{formatMoney(month.totalUSD, currency)}</strong>
          <small>Rest of the month at the current {formatMoney(month.runRateHourly * hoursPerMonth, currency)}/mo rate</small>
        </div>
        <div className={`cluster-metric ${budgetTone}`}>
          <span>Monthly budget</span>
          <strong>{budget ? formatMoney(budget.monthlyUSD, currency) : "Not set"}</strong>
          <small>{budget ? budgetSummary(budget, currency) : "Set one below to track the forecast against it"}</small>
        </div>
        <div className="cluster-metric neutral">
          <span>Days left in {monthName}</span>
          <strong>{daysLeft}</strong>
          <small>{budget ? paceSummary(budget, spent, daysLeft, currency) : `${formatMoney(month.projectedUSD, currency)} still to be billed`}</small>
        </div>
      </div>

      <section className="cluster-panel">
        <div className="cluster-panel-heading">
          <h2>Month to date</h2>
          <BudgetEditor clusterId={clusterId} current={budget?.monthlyUSD ?? 0} currency={currency} onSaved={() => setReload((value) => value + 1)} />
        </div>
        {budget && budget.state !== "on-track" && (
          <div className="budget-alert"><AlertCircle size={15} /> {budgetAlert(budget, month.totalUSD, currency)}</div>
        )}
        <BudgetProgress forecast={forecast} currency={currency} />
        <DailyForecastChart forecast={forecast} currency={currency} />
        <div className="panel-footnote">
          <CalendarClock size={14} /> Recorded spend comes from captures taken while the app runs. Hours without a capture are estimated at the average recorded rate, and the rest of the month at the current run rate. Figures are list prices for the calendar month.
        </div>
      </section>

      <WhatChanged clusterId={clusterId} currency={currency} capturedAt={String(report.generatedAt)} />
    </div>
  );
}

function budgetSummary(budget: NonNullable<CostForecast["budget"]>, currency: string) {
  if (budget.state === "exceeded") return `Already exceeded; forecast ${formatMoney(-budget.remainingUSD, currency)} over`;
  if (budget.remainingUSD < 0) return `Forecast ${formatMoney(-budget.remainingUSD, currency)} over (${Math.round(budget.projectedRatio * 100)}%)`;
  return `${formatMoney(budget.remainingUSD, currency)} left after the forecast (${Math.round(budget.projectedRatio * 100)}% used)`;
}

function paceSummary(budget: NonNullable<CostForecast["budget"]>, spent: number, daysLeft: number, currency: string) {
  if (budget.exhaustedAt) return `Budget runs out on ${new Date(budget.exhaustedAt).toLocaleDateString(undefined, { month: "short", day: "numeric" })}`;
  const left = budget.monthlyUSD - spent;
  if (left <= 0 || daysLeft <= 0) return "No budget left this month";
  return `Stay under ${formatMoney(left / daysLeft, currency)}/day to meet the budget`;
}

function budgetAlert(budget: NonNullable<CostForecast["budget"]>, total: number, currency: string) {
  if (budget.state === "exceeded") return `This month's spend has already passed the ${formatMoney(budget.monthlyUSD, currency)} budget.`;
  if (budget.state === "over") return `At the current run rate this month ends at ${formatMoney(total, currency)}, above the ${formatMoney(budget.monthlyUSD, currency)} budget.`;
  return `The forecast uses ${Math.round(budget.projectedRatio * 100)}% of the budget. Little room is left for growth this month.`;
}

function BudgetEditor({ clusterId, current, currency, onSaved }: { clusterId: string; current: number; currency: string; onSaved: () => void }) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const save = async (amount: number) => {
    setSaving(true);
    setError("");
    try {
      await setCostBudget(clusterId, amount);
      invalidateCostForecasts(clusterId);
      setEditing(false);
      onSaved();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setSaving(false);
    }
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const amount = Number(value);
    if (!Number.isFinite(amount) || amount <= 0) {
      setError("Enter a monthly amount above zero.");
      return;
    }
    void save(amount);
  };

  if (!editing) {
    return (
      <div className="budget-editor">
        <button type="button" className="link-button" onClick={() => { setValue(current > 0 ? String(current) : ""); setEditing(true); }}>
          <Wallet size={13} /> {current > 0 ? "Change budget" : "Set a monthly budget"}
        </button>
        {current > 0 && <button type="button" className="link-button muted" disabled={saving} onClick={() => void save(0)}>Remove</button>}
      </div>
    );
  }

  return (
    <form className="budget-editor" onSubmit={submit}>
      <label>
        <span>{currency} / month</span>
        <input type="number" min="1" step="any" value={value} autoFocus onChange={(event) => setValue(event.target.value)} aria-label="Monthly budget" />
      </label>
      <button type="submit" className="primary-button compact" disabled={saving}>{saving ? "Saving..." : "Save"}</button>
      <button type="button" className="link-button muted" onClick={() => setEditing(false)}>Cancel</button>
      {error && <small className="budget-editor-error">{error}</small>}
    </form>
  );
}

function BudgetProgress({ forecast, currency }: { forecast: CostForecast; currency: string }) {
  const month = forecast.forecast;
  const budget = forecast.budget?.monthlyUSD ?? 0;
  const scale = Math.max(month.totalUSD, budget) * 1.04;
  if (scale <= 0) return null;
  const width = (value: number) => `${(value / scale) * 100}%`;
  const parts = [
    { key: "recorded", label: "Recorded", value: month.recordedUSD, className: "", style: { background: recordedColor } },
    { key: "estimated", label: "Estimated (uncaptured hours)", value: month.estimatedUSD, className: "hatched-estimated", style: {} },
    { key: "projected", label: "Projected (rest of month)", value: month.projectedUSD, className: "hatched-projected", style: {} },
  ].filter((part) => part.value > 0);

  return (
    <div className="budget-progress">
      <div className="budget-track" role="img" aria-label={`${parts.map((part) => `${part.label} ${formatMoney(part.value, currency)}`).join(", ")}${budget > 0 ? `, budget ${formatMoney(budget, currency)}` : ""}`}>
        {parts.map((part) => (
          <span key={part.key} className={part.className} style={{ ...part.style, width: width(part.value) }} />
        ))}
        {budget > 0 && <i className="budget-marker" style={{ left: width(budget) }}><b>Budget</b></i>}
      </div>
      <div className="capacity-legend">
        {parts.map((part) => (
          <span key={part.key}><i className={part.className} style={part.style} /> {part.label} · {formatMoney(part.value, currency)}</span>
        ))}
      </div>
    </div>
  );
}

function DailyForecastChart({ forecast, currency }: { forecast: CostForecast; currency: string }) {
  const [focused, setFocused] = useState<number | null>(null);
  const days = forecast.forecast.days ?? [];
  const highest = days.reduce((maximum, day) => Math.max(maximum, day.recordedUSD + day.estimatedUSD + day.projectedUSD), 0);
  const budget = forecast.budget?.monthlyUSD ?? 0;
  const dailyBudget = budget > 0 && days.length > 0 ? budget / days.length : 0;
  const ceiling = Math.max(highest, dailyBudget) * 1.08;
  if (ceiling <= 0) return null;

  const label = (start: unknown) => new Date(start as string).toLocaleDateString(undefined, { month: "short", day: "numeric" });
  const active = focused !== null ? days[focused] : null;

  return (
    <div className="daily-forecast">
      <div className="bill-readout" aria-live="polite">
        {active
          ? <>{label(active.start)} · <strong>{formatMoney(active.recordedUSD + active.estimatedUSD + active.projectedUSD, currency)}</strong>
            {active.recordedUSD > 0 && <span className="trend-readout-part">{formatMoney(active.recordedUSD, currency)} recorded</span>}
            {active.estimatedUSD > 0 && <span className="trend-readout-part">{formatMoney(active.estimatedUSD, currency)} estimated</span>}
            {active.projectedUSD > 0 && <span className="trend-readout-part">{formatMoney(active.projectedUSD, currency)} projected</span>}</>
          : <>Daily spend this month{dailyBudget > 0 && <> · dashed line is the even daily share of the budget ({formatMoney(dailyBudget, currency)}/day)</>}</>}
      </div>
      <div className="trend-chart daily-chart" role="img" aria-label="Daily spend this month: recorded, estimated and projected">
        {dailyBudget > 0 && <i className="daily-budget-line" style={{ bottom: `${(dailyBudget / ceiling) * 100}%` }} />}
        {days.map((day, index) => {
          const total = day.recordedUSD + day.estimatedUSD + day.projectedUSD;
          return (
            <div key={String(day.start)} className={`trend-column ${focused !== null && focused !== index ? "dimmed" : ""}`} onMouseEnter={() => setFocused(index)} onMouseLeave={() => setFocused(null)}>
              <div className="trend-stack" style={{ height: `${(total / ceiling) * 100}%` }}>
                {day.projectedUSD > 0 && <span className="hatched-projected" style={{ flexGrow: day.projectedUSD }} />}
                {day.estimatedUSD > 0 && <span className="hatched-estimated" style={{ flexGrow: day.estimatedUSD }} />}
                {day.recordedUSD > 0 && <span style={{ flexGrow: day.recordedUSD, background: recordedColor }} />}
              </div>
            </div>
          );
        })}
      </div>
      <div className="trend-axis">
        {days.map((day, index) => <span key={String(day.start)}>{index % 5 === 0 ? label(day.start) : ""}</span>)}
      </div>
    </div>
  );
}

const changeWindows = historyWindows;

const driverLabels: Record<string, string> = {
  namespace: "Namespace",
  nodeGroup: "Node group",
  idle: "Idle",
  shared: "Shared",
};

function WhatChanged({ clusterId, currency, capturedAt }: { clusterId: string; currency: string; capturedAt: string }) {
  const [windowIndex, setWindowIndex] = useState<number>(defaultHistoryWindow);
  const selected = changeWindows[windowIndex];
  const { trend, error } = useCostTrend(clusterId, selected.days, selected.bucket, capturedAt);

  const drivers = trend?.drivers ?? [];
  const consumption = drivers.filter((driver) => driver.dimension !== "nodeGroup");
  const infrastructure = drivers.filter((driver) => driver.dimension === "nodeGroup");
  const series = trend?.series;
  const now = series && series.coveredHours > 0 ? series.totalProvisionedUSD / series.coveredHours : 0;
  const before = trend && trend.previous.coveredHours > 0 ? trend.previous.provisionedUSD / trend.previous.coveredHours : 0;

  return (
    <section className="cluster-panel" id="what-changed">
      <div className="cluster-panel-heading">
        <h2>What changed?</h2>
        <Segmented label="Comparison range" value={windowIndex} onChange={setWindowIndex} options={changeWindows.map((option, index) => ({ value: index, label: option.label, title: `Last ${option.label} against the ${option.previous}` }))} />
      </div>

      {error && <div className="panel-footnote"><AlertCircle size={14} /> {error}</div>}
      {!error && !trend && <div className="panel-footnote"><LoaderCircle className="spin" size={14} /> Comparing recorded captures...</div>}
      {trend && drivers.length === 0 && (
        <div className="panel-footnote"><AlertCircle size={14} /> Not enough captures to compare the last {selected.label} with the {selected.previous}. Each window needs at least an hour of recorded data.</div>
      )}
      {trend && drivers.length > 0 && (
        <>
          <p className="change-headline">
            Average billed rate {now >= before ? "rose" : "fell"} from <strong>{formatMoney(before * hoursPerMonth, currency)}/mo</strong> to <strong>{formatMoney(now * hoursPerMonth, currency)}/mo</strong>
            {before > 0 && <> ({now >= before ? "+" : "−"}{Math.abs(((now - before) / before) * 100).toFixed(1)}%)</>} compared with the {selected.previous}.
          </p>
          <div className="change-groups">
            <DriverList title="Who consumed it" hint="Namespaces, idle and shared add up to the change in the bill." drivers={consumption} currency={currency} />
            <DriverList title="What was billed" hint="Node cost per node group. The same money seen from the infrastructure side." drivers={infrastructure} currency={currency} />
          </div>
        </>
      )}
      <div className="panel-footnote">
        <TrendingUp size={14} /> Changes compare average hourly rates over the captured hours of each window, shown as a monthly amount, so windows with different coverage stay comparable.
      </div>
    </section>
  );
}

function DriverList({ title, hint, drivers, currency }: { title: string; hint: string; drivers: CostDriver[]; currency: string }) {
  const shown = drivers.slice(0, 6);
  const largest = shown.reduce((maximum, driver) => Math.max(maximum, Math.abs(driver.delta.monthly)), 0);
  return (
    <div className="change-group">
      <h3>{title}</h3>
      <small>{hint}</small>
      {shown.length === 0
        ? <div className="panel-footnote standalone">No change.</div>
        : <div className="driver-list">
          {shown.map((driver) => {
            const up = driver.delta.monthly > 0;
            return (
              <div className="driver-row" key={`${driver.dimension}/${driver.key}`}>
                <div>
                  <span className="driver-name"><strong>{driver.key}</strong><small>{driverLabels[driver.dimension] ?? driver.dimension} · {formatMoney(driver.previous.monthly, currency)} → {formatMoney(driver.current.monthly, currency)}/mo</small></span>
                  <b>{up ? <TrendingUp size={13} /> : <TrendingDown size={13} />} {up ? "+" : "−"}{formatMoney(Math.abs(driver.delta.monthly), currency)}/mo</b>
                </div>
                <div className="capacity-track"><span style={{ width: `${largest > 0 ? (Math.abs(driver.delta.monthly) / largest) * 100 : 0}%`, background: up ? increaseColor : decreaseColor }} /></div>
              </div>
            );
          })}
        </div>}
    </div>
  );
}
