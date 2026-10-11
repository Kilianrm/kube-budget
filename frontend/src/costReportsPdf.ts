import type { jsPDF as JsPdfDocument } from "jspdf";
import type { CostForecast, CostReport, OptimizationResult } from "./backend";
import { formatMoney } from "./costFormat";

type PdfWithTables = JsPdfDocument & { lastAutoTable?: { finalY: number } };

async function createReport(title: string, capturedAt: string) {
  const [{ jsPDF }, { autoTable }] = await Promise.all([import("jspdf"), import("jspdf-autotable")]);
  const pdf = new jsPDF();
  const margin = 16;
  const pageWidth = pdf.internal.pageSize.getWidth();
  const pageHeight = pdf.internal.pageSize.getHeight();
  const contentWidth = pageWidth - margin * 2;
  let cursorY = 18;

  pdf.setProperties({ title: `Kube Cost Workspace ${title}`, subject: "Kubernetes cost report", creator: "Kube Cost Workspace" });
  pdf.setFont("helvetica", "bold");
  pdf.setFontSize(19);
  pdf.setTextColor(23, 37, 31);
  pdf.text("Kube Cost Workspace", margin, cursorY);
  cursorY += 8;
  pdf.setFontSize(13);
  pdf.setTextColor(28, 53, 73);
  pdf.text(title, margin, cursorY);
  cursorY += 7;
  pdf.setFont("helvetica", "normal");
  pdf.setFontSize(8);
  pdf.setTextColor(102, 115, 109);
  pdf.text(`Cost report captured ${formatTimestamp(capturedAt)}. List-price estimate; not a provider invoice.`, margin, cursorY);
  cursorY += 10;

  const addHeading = (heading: string) => {
    if (cursorY > pageHeight - 26) { pdf.addPage(); cursorY = 18; }
    pdf.setFont("helvetica", "bold");
    pdf.setFontSize(11);
    pdf.setTextColor(28, 53, 73);
    pdf.text(heading, margin, cursorY);
    cursorY += 5;
  };

  const addTable = (headers: string[], rows: string[][]) => {
    if (rows.length === 0) return;
    autoTable(pdf, {
      startY: cursorY,
      margin: { left: margin, right: margin, bottom: 16 },
      head: [headers],
      body: rows,
      theme: "grid",
      styles: { font: "helvetica", fontSize: 8, cellPadding: 2.5, textColor: [41, 68, 87], lineColor: [217, 224, 221], lineWidth: 0.15, overflow: "linebreak" },
      headStyles: { fillColor: [243, 247, 246], textColor: [69, 86, 78], fontStyle: "bold" },
      alternateRowStyles: { fillColor: [250, 252, 251] },
    });
    cursorY = ((pdf as PdfWithTables).lastAutoTable?.finalY ?? cursorY) + 8;
  };

  const addNote = (text: string) => {
    if (!text) return;
    pdf.setFont("helvetica", "normal");
    pdf.setFontSize(8);
    pdf.setTextColor(102, 115, 109);
    const lines = pdf.splitTextToSize(text, contentWidth);
    for (const line of lines) {
      if (cursorY > pageHeight - 16) { pdf.addPage(); cursorY = 18; }
      pdf.text(line, margin, cursorY);
      cursorY += 4;
    }
    cursorY += 3;
  };

  const save = (filename: string) => {
    const pageCount = pdf.getNumberOfPages();
    for (let page = 1; page <= pageCount; page++) {
      pdf.setPage(page);
      pdf.setFont("helvetica", "normal");
      pdf.setFontSize(8);
      pdf.setTextColor(120, 132, 127);
      pdf.text(`Kube Cost Workspace / ${title}`, margin, pageHeight - 8);
      pdf.text(`${page} / ${pageCount}`, pageWidth - margin, pageHeight - 8, { align: "right" });
    }
    pdf.save(filename);
  };

  return { pdf, margin, cursor: () => cursorY, setCursor: (value: number) => { cursorY = value; }, addHeading, addTable, addNote, save };
}

export async function downloadBudgetForecastPdf(report: CostReport, forecast: CostForecast) {
  const currency = report.currency || "USD";
  const month = forecast.forecast;
  const budget = forecast.budget;
  const monthName = new Date(month.monthStart).toLocaleString(undefined, { month: "long", year: "numeric" });
  const spent = month.recordedUSD + month.estimatedUSD;
  const coverage = month.elapsedHours > 0 ? Math.round((month.coveredHours / month.elapsedHours) * 100) : 0;
  const daysLeft = Math.ceil(month.remainingHours / 24);
  const pdf = await createReport(`Budget & forecast - ${monthName}`, String(report.generatedAt));

  pdf.addHeading("Monthly outlook");
  pdf.addTable(["Measure", "Amount / status"], [
    ["Recorded spend", formatMoney(month.recordedUSD, currency)],
    ["Estimated spend for uncaptured hours", formatMoney(month.estimatedUSD, currency)],
    ["Spent to date", formatMoney(spent, currency)],
    ["Projected for the rest of the month", formatMoney(month.projectedUSD, currency)],
    ["Month-end forecast", formatMoney(month.totalUSD, currency)],
    ["Current monthly run rate", formatMoney(month.runRateHourly * 730, currency)],
    ["Monthly budget", budget ? formatMoney(budget.monthlyUSD, currency) : "Not set"],
    ["Budget status", budget ? budget.state : "No budget configured"],
    ...(budget ? [["Forecast vs budget", `${formatMoney(Math.abs(month.totalUSD - budget.monthlyUSD), currency)} ${month.totalUSD > budget.monthlyUSD ? "over" : "under"}`]] : []),
    ["Days remaining", String(daysLeft)],
    ["Capture coverage", `${coverage}% (${month.coveredHours.toFixed(1)} of ${month.elapsedHours.toFixed(1)} elapsed hours)`],
  ]);

  pdf.addHeading("Daily spend breakdown");
  pdf.addTable(["Date", "Recorded", "Uncaptured estimate", "Rest-of-month projection", "Daily total"], (month.days ?? []).map(day => [
    new Date(day.start).toLocaleDateString(undefined, { month: "short", day: "numeric" }),
    formatMoney(day.recordedUSD, currency), formatMoney(day.estimatedUSD, currency), formatMoney(day.projectedUSD, currency),
    formatMoney(day.recordedUSD + day.estimatedUSD + day.projectedUSD, currency),
  ]));

  pdf.addHeading("Assumptions and data quality");
  pdf.addNote("Hours without a capture are estimated at the average recorded rate. The remaining days use the current run rate. Figures use published list prices for the calendar month; discounts and unpriced services may change actual spend.");
  for (const assumption of report.assumptions ?? []) pdf.addNote(assumption.detail);
  for (const warning of report.warnings ?? []) pdf.addNote(`${warning.subject ? `${warning.subject}: ` : ""}${warning.message}`);

  pdf.save(`kube-cost-workspace-budget-forecast-${new Date().toISOString().slice(0, 10)}.pdf`);
}

export async function downloadOptimizationPdf(report: CostReport, optimization: OptimizationResult, budget?: number) {
  const currency = report.currency || "USD";
  const plan = optimization.plan;
  const savingsShare = plan.current.monthly > 0 ? Math.round((plan.savings.monthly / plan.current.monthly) * 100) : 0;
  const applied = new Map((optimization.applied ?? []).map(entry => [entry.id, entry]));
  const pdf = await createReport("Optimization plan", String(report.generatedAt));

  pdf.addHeading("Plan summary");
  pdf.addTable(["Measure", "Monthly amount / status"], [
    ["Current infrastructure bill", formatMoney(plan.current.monthly, currency)],
    ["Achievable savings", formatMoney(plan.savings.monthly, currency)],
    ["Share of current bill", `${savingsShare}%`],
    ["Estimated bill after plan", formatMoney(plan.optimized.monthly, currency)],
    ["Monthly budget", budget === undefined ? "Not set" : formatMoney(budget, currency)],
    ["Open recommendations", String(plan.recommendations.filter(item => !item.dismissed && !applied.get(item.id)?.pending).length)],
  ]);

  pdf.addHeading("Savings plan");
  pdf.addTable(["Step", "Status", "Monthly savings", "Already achieved"], plan.steps.map(step => [
    step.title, step.status ?? "Open", formatMoney(step.savings.monthly, currency), formatMoney(step.achieved.monthly, currency),
  ]));
  if (plan.steps.length === 0) pdf.addNote("No savings steps are available in this report.");

  pdf.addHeading("Recommendations");
  pdf.addTable(["Recommendation", "Category", "Risk / effort", "Status", "Monthly savings", "Action"], plan.recommendations.map(item => {
    const appliedEntry = applied.get(item.id);
    const status = item.dismissed ? "Dismissed" : appliedEntry?.pending ? "In progress" : appliedEntry ? "Confirmed" : "Open";
    return [item.title, item.category, `${item.risk} / ${item.effort}`, status, formatMoney(item.savings.monthly, currency), item.action];
  }));

  if ((plan.dataIssues ?? []).length > 0) {
    pdf.addHeading("Data issues to resolve first");
    pdf.addTable(["Issue", "Why it matters", "Suggested action"], plan.dataIssues.map(issue => [issue.title, issue.rationale, issue.action]));
  }
  if ((optimization.history ?? []).length > 0) {
    pdf.addHeading("Recommendation activity");
    pdf.addTable(["Date", "Recommendation", "Activity", "Expected savings / month"], optimization.history.map(event => [
      formatTimestamp(event.at), event.title, event.action, formatMoney(event.savings.monthly, currency),
    ]));
  }

  pdf.addHeading("Assumptions and limitations");
  for (const assumption of plan.assumptions ?? []) pdf.addNote(assumption.detail);
  if (plan.paused) pdf.addNote(`Some node recommendations are paused: ${plan.paused}`);
  pdf.addNote("Savings are estimates at published list prices and are not measured until a change is applied and a later cost report confirms its impact. Recommendations can be limited by missing or incomplete cluster data.");
  for (const warning of report.warnings ?? []) pdf.addNote(`${warning.subject ? `${warning.subject}: ` : ""}${warning.message}`);

  pdf.save(`kube-cost-workspace-optimization-plan-${new Date().toISOString().slice(0, 10)}.pdf`);
}

function formatTimestamp(value: string | Date) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}