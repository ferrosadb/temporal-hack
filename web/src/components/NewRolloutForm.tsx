import { Rocket, X } from "lucide-react";
import { useState } from "react";
import { api } from "@/lib/api";

type Props = {
  cohort: string[];
  onClear: (id: string) => void;
  onSuccess: (rolloutId: string) => void;
};

export function NewRolloutForm({ cohort, onClear, onSuccess }: Props) {
  const [imageRef, setImageRef] = useState("");
  const [smoke, setSmoke] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const disabled = submitting || cohort.length === 0 || imageRef.trim() === "";

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const res = await api.startRollout({
        image_ref: imageRef.trim(),
        smoke_command: smoke.trim() || undefined,
        cohort_selector: { robot_ids: cohort },
      });
      onSuccess(res.rollout_id);
      setImageRef("");
      setSmoke("");
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <aside className="rounded-lg border border-border bg-card/60 backdrop-blur">
      <header className="border-b border-border px-5 py-4">
        <h2 className="text-sm font-semibold tracking-tight flex items-center gap-2">
          <Rocket className="h-4 w-4 text-primary" />
          New Rollout
        </h2>
      </header>

      <form onSubmit={submit} className="px-5 py-4 space-y-4">
        <Field label="Image ref">
          <input
            type="text"
            value={imageRef}
            onChange={(e) => setImageRef(e.target.value)}
            placeholder="registry/robot-app:v2"
            className="w-full rounded-md bg-input border border-border px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary/60"
          />
        </Field>

        <Field label="Smoke command">
          <input
            type="text"
            value={smoke}
            onChange={(e) => setSmoke(e.target.value)}
            placeholder="true"
            className="w-full rounded-md bg-input border border-border px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary/60"
          />
        </Field>

        <Field label={`Cohort (${cohort.length})`}>
          {cohort.length === 0 ? (
            <div className="rounded-md border border-dashed border-border px-3 py-3 text-xs text-muted-foreground">
              Select robots from the table →
            </div>
          ) : (
            <ul className="flex flex-wrap gap-1.5">
              {cohort.map((id) => (
                <li
                  key={id}
                  className="inline-flex items-center gap-1.5 rounded-full bg-primary/10 border border-primary/30 px-2 py-1 text-xs font-mono text-primary"
                >
                  {id}
                  <button
                    type="button"
                    onClick={() => onClear(id)}
                    className="opacity-70 hover:opacity-100"
                    aria-label={`Remove ${id}`}
                  >
                    <X className="h-3 w-3" />
                  </button>
                </li>
              ))}
            </ul>
          )}
        </Field>

        {error && (
          <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive">
            {error}
          </div>
        )}

        <button
          type="submit"
          disabled={disabled}
          className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground hover:bg-primary/90 transition disabled:cursor-not-allowed disabled:opacity-50"
        >
          <Rocket className="h-4 w-4" />
          {submitting
            ? "Deploying…"
            : `Deploy to ${cohort.length} robot${cohort.length === 1 ? "" : "s"}`}
        </button>
      </form>
    </aside>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="text-xs font-medium text-muted-foreground mb-1.5 block">{label}</span>
      {children}
    </label>
  );
}
