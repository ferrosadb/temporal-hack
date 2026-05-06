import { useCallback, useEffect, useState } from "react";
import { Header } from "@/components/Header";
import { FleetTable } from "@/components/FleetTable";
import { NewRolloutForm } from "@/components/NewRolloutForm";
import { api, type RobotRow } from "@/lib/api";

const REFRESH_MS = 5_000;

export default function App() {
  const [robots, setRobots] = useState<RobotRow[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [toast, setToast] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const rows = await api.listRobots();
      setRobots(rows ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load robots");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, REFRESH_MS);
    return () => clearInterval(id);
  }, [refresh]);

  const toggle = useCallback((id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const toggleAll = useCallback(() => {
    setSelected((prev) =>
      prev.size === robots.length ? new Set() : new Set(robots.map((r) => r.robot_id)),
    );
  }, [robots]);

  const onSuccess = useCallback((rolloutId: string) => {
    setToast(`Rollout ${rolloutId} started`);
    setSelected(new Set());
    setTimeout(() => setToast(null), 4000);
  }, []);

  return (
    <div className="min-h-screen">
      <Header apiUrl={api.baseUrl()} />

      <main className="container py-8 grid gap-6 lg:grid-cols-[1fr_360px]">
        <FleetTable
          robots={robots}
          selected={selected}
          loading={loading}
          error={error}
          onToggle={toggle}
          onToggleAll={toggleAll}
          onRefresh={refresh}
        />
        <NewRolloutForm
          cohort={Array.from(selected)}
          onClear={(id) =>
            setSelected((prev) => {
              const next = new Set(prev);
              next.delete(id);
              return next;
            })
          }
          onSuccess={onSuccess}
        />
      </main>

      {toast && (
        <div className="fixed bottom-6 right-6 rounded-md border border-primary/40 bg-card/95 px-4 py-3 text-sm shadow-lg shadow-primary/10 backdrop-blur">
          {toast}
        </div>
      )}
    </div>
  );
}
