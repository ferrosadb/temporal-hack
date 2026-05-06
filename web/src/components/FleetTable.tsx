import { RefreshCw } from "lucide-react";
import { type RobotRow } from "@/lib/api";
import { cn, relativeTime, status } from "@/lib/utils";

type Props = {
  robots: RobotRow[];
  selected: Set<string>;
  loading: boolean;
  error: string | null;
  onToggle: (id: string) => void;
  onToggleAll: () => void;
  onRefresh: () => void;
};

export function FleetTable({
  robots,
  selected,
  loading,
  error,
  onToggle,
  onToggleAll,
  onRefresh,
}: Props) {
  const allSelected = robots.length > 0 && robots.every((r) => selected.has(r.robot_id));

  return (
    <section className="rounded-lg border border-border bg-card/60 backdrop-blur">
      <header className="flex items-center justify-between border-b border-border px-5 py-4">
        <div>
          <h2 className="text-sm font-semibold tracking-tight">Fleet</h2>
          <div className="text-xs text-muted-foreground mt-0.5">
            {robots.length} robots • {selected.size} selected
          </div>
        </div>
        <button
          type="button"
          onClick={onRefresh}
          disabled={loading}
          className="inline-flex items-center gap-2 rounded-md border border-border bg-card px-3 py-1.5 text-xs font-medium hover:bg-accent/30 hover:border-primary/40 transition disabled:opacity-50"
        >
          <RefreshCw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
          Refresh
        </button>
      </header>

      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-xs uppercase tracking-wide text-muted-foreground">
              <th className="px-5 py-3 text-left font-medium w-10">
                <input
                  type="checkbox"
                  aria-label="Select all"
                  checked={allSelected}
                  onChange={onToggleAll}
                  className="h-4 w-4 rounded border-border bg-input accent-primary"
                />
              </th>
              <th className="px-5 py-3 text-left font-medium">Robot ID</th>
              <th className="px-5 py-3 text-left font-medium">Last seen</th>
              <th className="px-5 py-3 text-left font-medium">Buffered</th>
              <th className="px-5 py-3 text-left font-medium">Status</th>
            </tr>
          </thead>
          <tbody>
            {error ? (
              <tr>
                <td colSpan={5} className="px-5 py-10 text-center text-sm text-destructive">
                  {error}
                </td>
              </tr>
            ) : robots.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-5 py-10 text-center text-sm text-muted-foreground">
                  {loading ? "Loading…" : "No robots reporting."}
                </td>
              </tr>
            ) : (
              robots.map((r) => {
                const st = (r.status || status(r.last_seen)) as "online" | "stale";
                const isSelected = selected.has(r.robot_id);
                const buffered =
                  r.buffered_samples && r.buffered_samples > 0 ? r.buffered_samples.toLocaleString() : "—";
                return (
                  <tr
                    key={r.robot_id}
                    onClick={() => onToggle(r.robot_id)}
                    className={cn(
                      "border-t border-border/60 cursor-pointer transition",
                      isSelected ? "bg-primary/10" : "hover:bg-accent/20",
                    )}
                  >
                    <td className="px-5 py-3" onClick={(e) => e.stopPropagation()}>
                      <input
                        type="checkbox"
                        aria-label={`Select ${r.robot_id}`}
                        checked={isSelected}
                        onChange={() => onToggle(r.robot_id)}
                        className="h-4 w-4 rounded border-border bg-input accent-primary"
                      />
                    </td>
                    <td className="px-5 py-3 font-mono text-xs">{r.robot_id}</td>
                    <td className="px-5 py-3 text-muted-foreground">
                      {relativeTime(r.last_seen)}
                    </td>
                    <td className="px-5 py-3 text-muted-foreground">{buffered}</td>
                    <td className="px-5 py-3">
                      <StatusPill status={st} />
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function StatusPill({ status }: { status: "online" | "stale" }) {
  const isOnline = status === "online";
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium",
        isOnline
          ? "bg-primary/10 text-primary border border-primary/30"
          : "bg-muted/40 text-muted-foreground border border-border",
      )}
    >
      <span
        className={cn(
          "h-1.5 w-1.5 rounded-full",
          isOnline ? "bg-primary shadow-[0_0_8px_currentColor]" : "bg-muted-foreground",
        )}
      />
      {isOnline ? "online" : "stale"}
    </span>
  );
}
