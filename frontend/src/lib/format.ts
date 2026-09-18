/**
 * Formats a ruble amount into "0.00 ₽" notation
 */
export function formatRub(amount: number | null | undefined): string {
  if (amount === null || amount === undefined || isNaN(amount)) {
    return "0.00 ₽";
  }
  return `${amount.toFixed(2)} ₽`;
}

/**
 * Formats cost in kopecks to rubles "0.00 ₽"
 */
export function formatKopecks(kopecks: number | null | undefined): string {
  if (!kopecks || isNaN(kopecks)) {
    return "0.00 ₽";
  }
  return formatRub(kopecks / 100);
}

/**
 * Formats an ISO date string into a user-friendly Russian format
 */
export function formatDateTime(dateStr: string | null | undefined): string {
  if (!dateStr) return "";
  const date = new Date(dateStr);
  if (isNaN(date.getTime())) return "";

  const now = new Date();
  const isToday =
    date.getDate() === now.getDate() &&
    date.getMonth() === now.getMonth() &&
    date.getFullYear() === now.getFullYear();

  const hours = date.getHours().toString().padStart(2, "0");
  const minutes = date.getMinutes().toString().padStart(2, "0");
  const timeStr = `${hours}:${minutes}`;

  if (isToday) {
    return timeStr;
  }

  const yesterday = new Date(now);
  yesterday.setDate(now.getDate() - 1);
  const isYesterday =
    date.getDate() === yesterday.getDate() &&
    date.getMonth() === yesterday.getMonth() &&
    date.getFullYear() === yesterday.getFullYear();

  if (isYesterday) {
    return `Вчера, ${timeStr}`;
  }

  const day = date.getDate().toString().padStart(2, "0");
  const month = (date.getMonth() + 1).toString().padStart(2, "0");
  return `${day}.${month}.${date.getFullYear()}`;
}

/**
 * Formats token numbers (e.g. 128000 -> "128K")
 */
export function formatTokens(tokens: number | null | undefined): string {
  if (!tokens || isNaN(tokens)) return "0";
  if (tokens >= 1_000_000) {
    return `${(tokens / 1_000_000).toFixed(1)}M`;
  }
  if (tokens >= 1_000) {
    return `${Math.round(tokens / 1_000)}K`;
  }
  return tokens.toLocaleString("ru-RU");
}
