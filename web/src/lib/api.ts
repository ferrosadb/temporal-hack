// API client. The Vite dev server proxies /v1/* to CONTROLPLANE_URL
// (default http://localhost:8081). In production builds this falls
// back to same-origin so the bundle can be served from the
// controlplane itself.

export type RobotRow = {
  robot_id: string;
  last_seen: string;
};

export type RolloutSpec = {
  image_ref: string;
  smoke_command?: string;
  cohort_selector: { robot_ids: string[] };
};

export type StartRolloutResponse = {
  rollout_id: string;
};

const baseUrl = (() => {
  const fromStorage =
    typeof window !== "undefined"
      ? window.localStorage.getItem("robot-cp-base")
      : null;
  if (fromStorage) return fromStorage.replace(/\/+$/, "");
  return "";
})();

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(baseUrl + path, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok) {
    const body = await res.text().catch(() => "");
    throw new Error(`${res.status} ${res.statusText}: ${body || path}`);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  baseUrl: () => baseUrl || window.location.origin,

  listRobots: () => req<RobotRow[]>("/v1/robots"),

  startRollout: (spec: RolloutSpec) =>
    req<StartRolloutResponse>("/v1/ota/rollouts", {
      method: "POST",
      body: JSON.stringify(spec),
    }),
};
