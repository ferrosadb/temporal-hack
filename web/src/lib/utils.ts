export function cn(...classes: (string | false | null | undefined)[]): string {
  return classes.filter(Boolean).join(" ");
}

// Status threshold: a robot whose last heartbeat is within this many
// seconds is considered online. Beyond it the row turns muted/stale.
const ONLINE_WITHIN_SEC = 60;

export function relativeTime(iso: string): string {
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return iso;
  const seconds = Math.floor((Date.now() - t) / 1000);
  if (seconds < 5) return "just now";
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

export function status(lastSeenIso: string): "online" | "stale" {
  const t = Date.parse(lastSeenIso);
  if (Number.isNaN(t)) return "stale";
  return (Date.now() - t) / 1000 < ONLINE_WITHIN_SEC ? "online" : "stale";
}
