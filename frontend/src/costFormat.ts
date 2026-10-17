/** Formats money with precision that fits its size: cents for small hourly
 * rates, whole dollars once amounts reach the thousands. */
export function formatMoney(value: number, currency = "USD") {
  const magnitude = Math.abs(value);
  const digits = magnitude === 0 || magnitude >= 1000 ? 0 : magnitude < 1 ? 4 : 2;
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  }).format(value);
}

/** Hours in an average month, the same constant the backend projects with. */
export const hoursPerMonth = 730;
