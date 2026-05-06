import { Cpu } from "lucide-react";

export function Header({ apiUrl }: { apiUrl: string }) {
  return (
    <header className="border-b border-border bg-background/80 backdrop-blur supports-[backdrop-filter]:bg-background/60 sticky top-0 z-10">
      <div className="container flex h-16 items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-9 items-center justify-center rounded-md bg-primary/10 border border-primary/30 glow-primary">
            <Cpu className="h-4 w-4 text-primary" />
          </div>
          <div className="leading-tight">
            <div className="text-sm font-semibold tracking-tight">
              Robot Control Plane
            </div>
            <div className="text-xs text-muted-foreground">
              OTA rollout console
            </div>
          </div>
        </div>
        <div className="hidden md:flex items-center gap-2 text-xs text-muted-foreground">
          <span className="rounded-md border border-border bg-card px-2 py-1 font-mono">
            {apiUrl}
          </span>
        </div>
      </div>
    </header>
  );
}
