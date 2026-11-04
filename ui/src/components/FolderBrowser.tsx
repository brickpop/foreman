import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useClient } from "../api/client";
import { Button } from "./ui";

// Picks a folder on the CEO's disk (wiki/product/features/add-project.md#picking-repos):
// Foreman's folder browser, one folder at a time, never above the folder it was
// started in, or a pasted path. A browser can't tell where a dropped folder is.

const field =
  "min-w-0 flex-1 rounded-lg border border-line2 bg-term px-3 py-2 font-mono text-[13px] text-tx";

export function RepoPicker({
  pick,
  onPick,
}: {
  /** "repo": a git repo's top folder; "folder": the folder a new repo goes in. */
  pick: "repo" | "folder";
  onPick: (path: string) => void;
}) {
  const client = useClient();
  const [path, setPath] = useState<string>();
  const [typed, setTyped] = useState("");
  const folder = useQuery({
    queryKey: ["folders", path ?? ""],
    queryFn: () => client.home.folders(path),
  });
  const f = folder.data;
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <span className="min-w-0 flex-1 truncate font-mono text-[12.5px] text-tx3" title={f?.path}>
          {f?.path ?? "…"}
        </span>
        {pick === "folder" && f && (
          <Button variant="small" onClick={() => onPick(f.path)}>
            Here
          </Button>
        )}
      </div>
      <div className="flex max-h-[min(280px,36vh)] flex-col overflow-y-auto rounded-[9px] border border-line2">
        {folder.error && (
          <p className="m-0 px-3 py-2 text-[13px] text-err">{folder.error.message}</p>
        )}
        {f?.parent && <Row onOpen={() => setPath(f.parent)} name=".." />}
        {f?.entries.map((e) => (
          <Row
            key={e.path}
            name={e.name}
            repo={e.repo}
            onOpen={() => setPath(e.path)}
            onPick={pick === "repo" && e.repo ? () => onPick(e.path) : undefined}
          />
        ))}
        {f && !f.parent && f.entries.length === 0 && (
          <p className="m-0 px-3 py-2 text-[13px] text-tx4">No folders here.</p>
        )}
      </div>
      <form
        className="flex items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (typed.trim()) onPick(typed.trim());
        }}
      >
        <input
          value={typed}
          onChange={(e) => setTyped(e.target.value)}
          placeholder={pick === "repo" ? "or paste a repo's path" : "or paste a folder's path"}
          aria-label="Path"
          spellCheck={false}
          className={field}
        />
        <Button type="submit" variant="small" disabled={!typed.trim()}>
          Use
        </Button>
      </form>
    </div>
  );
}

function Row({
  name,
  repo,
  onOpen,
  onPick,
}: {
  name: string;
  repo?: boolean;
  onOpen: () => void;
  onPick?: () => void;
}) {
  return (
    <div className="flex items-center gap-2 border-b border-line px-3 py-1.5 last:border-b-0 hover:bg-hover">
      <button
        type="button"
        onClick={onOpen}
        className="min-w-0 flex-1 truncate text-left font-mono text-[13px] text-tx2"
      >
        {name}/
      </button>
      {repo && <span className="font-mono text-[11px] text-ok">git</span>}
      {onPick && (
        <Button variant="small" onClick={onPick}>
          Pick
        </Button>
      )}
    </div>
  );
}
