import { useEffect, useRef, useState } from "react";
import { type Instance, useClient } from "../api/client";
import { Terminal, type TerminalHandle } from "../components/Terminal";

// Walking-skeleton Sessions screen: one tab per session, one pane.
// The full screen is specified in wiki/product/features/sessions.md.
export function Sessions() {
  const client = useClient();
  const [instances, setInstances] = useState<Instance[]>();
  const [error, setError] = useState<string>();
  const [active, setActive] = useState<string>();
  const [copied, setCopied] = useState(false);
  const terminal = useRef<TerminalHandle>(null);

  useEffect(() => {
    client
      .instances()
      .then((list) => {
        setInstances(list);
        setActive((current) => current ?? list[0]?.id);
      })
      .catch((e: Error) => setError(e.message));
  }, [client]);

  const copy = async () => {
    await navigator.clipboard.writeText(terminal.current?.text() ?? "");
    setCopied(true);
    setTimeout(() => setCopied(false), 1400);
  };

  if (error) return <Message>Couldn't load the sessions: {error}</Message>;
  if (!instances) return <Message>…</Message>;
  if (!active) return <Message>No sessions yet.</Message>;

  return (
    <div className="flex h-full min-h-0 flex-col">
      <nav className="flex overflow-x-auto border-b border-line bg-side px-3">
        {instances.map(({ id }) => (
          <button
            key={id}
            type="button"
            onClick={() => setActive(id)}
            className={`cursor-pointer px-3.5 py-2.5 font-medium ${
              id === active ? "text-tx shadow-[inset_0_-2px_0_var(--acc)]" : "text-tx3"
            }`}
          >
            {id}
          </button>
        ))}
      </nav>
      <section className="flex min-h-0 flex-1 flex-col bg-term">
        <header className="flex items-center gap-3 border-b border-line px-4 py-2 text-[13px]">
          <span className="font-semibold">{active}</span>
          <span className="flex-1" />
          <button
            type="button"
            onClick={copy}
            className="cursor-pointer rounded-[5px] border border-line2 px-2 py-0.5 text-xs text-tx2"
          >
            {copied ? "Copied" : "Copy"}
          </button>
        </header>
        <div className="min-h-0 flex-1 px-4 py-3">
          <Terminal key={active} instance={active} ref={terminal} />
        </div>
      </section>
    </div>
  );
}

function Message({ children }: { children: React.ReactNode }) {
  return <p className="p-12 text-tx3">{children}</p>;
}
