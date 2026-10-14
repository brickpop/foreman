import { useEffect, useState } from "react";

type Workspace = { path: string };

// Walking-skeleton shell: the sidebar brand and the workspace the server reports.
export function App() {
  const [workspace, setWorkspace] = useState<Workspace>();
  const [error, setError] = useState<string>();

  useEffect(() => {
    fetch("/api/workspace")
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`HTTP ${r.status}`))))
      .then(setWorkspace)
      .catch((e: Error) => setError(e.message));
  }, []);

  return (
    <div className="grid h-screen grid-cols-[220px_minmax(0,1fr)]">
      <aside className="flex flex-col border-r border-line bg-side">
        <div className="flex items-center gap-[9px] px-[18px] pt-5 pb-4">
          <div className="grid size-[22px] place-items-center rounded-[5px] bg-acc font-mono text-xs font-bold text-accInk">
            F
          </div>
          <div className="text-[15px] font-semibold tracking-[-0.01em]">Foreman</div>
        </div>
      </aside>
      <main className="p-12">
        <h1 className="m-0 text-3xl font-semibold tracking-[-0.02em]">Workspace</h1>
        <p className="font-mono text-[12.5px] text-tx4">
          {error ? `Couldn't reach the server: ${error}` : (workspace?.path ?? "…")}
        </p>
      </main>
    </div>
  );
}
