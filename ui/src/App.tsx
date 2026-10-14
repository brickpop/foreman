import { Sessions } from "./screens/Sessions";

// Walking-skeleton shell: the sidebar brand and the Sessions screen.
// The full shell is specified in wiki/product/features/app-shell.md.
export function App() {
  return (
    <div className="grid h-screen grid-cols-[220px_minmax(0,1fr)]">
      <aside className="flex flex-col border-r border-line bg-side">
        <div className="flex items-center gap-[9px] px-[18px] pt-5 pb-4">
          <div className="grid size-[22px] place-items-center rounded-[5px] bg-acc font-mono text-xs font-bold text-accInk">
            F
          </div>
          <div className="text-[15px] font-semibold tracking-[-0.01em]">Foreman</div>
        </div>
        <nav className="px-2.5">
          <div className="rounded-md bg-raised px-2.5 py-2 font-medium text-tx">Sessions</div>
        </nav>
      </aside>
      <main className="min-h-0 min-w-0">
        <Sessions />
      </main>
    </div>
  );
}
