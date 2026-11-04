import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useClient } from "../api/client";
import { folderCrumbs } from "../app/addProject";
import { FolderIcon, ForemanFolderIcon, RepoIcon } from "./icons";

// Foreman's folder browser on the CEO's disk (wiki/product/features/add-project.md#picking-repos):
// one folder at a time, its path as breadcrumbs from the home folder. The folder shown is what's picked: whoever holds the browser
// reads it (a git repo's top folder, or where a new repo goes). A browser
// can't tell where a dropped folder is.

const folderQuery = (
  c: ReturnType<typeof useClient>,
  path: string | undefined,
  project: string | undefined,
) => ({
  queryKey: ["folders", path ?? "", project ?? ""],
  queryFn: () => c.home.folders(path, project),
});

/** The browser's state: the folder shown (the start folder at first), and
 * open, which shows another once it is read, so a bad path leaves it as it is.
 * For a project, each folder says which of its repos it already is. */
export function useFolderBrowser(project?: string) {
  const client = useClient();
  const queries = useQueryClient();
  const [path, setPath] = useState<string>();
  const folder = useQuery(folderQuery(client, path, project));
  const open = async (to: string) => {
    await queries.fetchQuery(folderQuery(client, to, project));
    setPath(to);
  };
  return { folder: folder.data, error: folder.error, open };
}

export function FolderBrowser({ browser }: { browser: ReturnType<typeof useFolderBrowser> }) {
  const { folder: f, open } = browser;
  const [failed, setFailed] = useState<string>();
  // A folder that can't be opened (gone since it was listed) says why.
  const go = (path: string) =>
    open(path).then(
      () => setFailed(undefined),
      (e: Error) => setFailed(e.message),
    );
  return (
    <div className="flex flex-col gap-2">
      <nav aria-label="Folder" className="flex min-h-7 flex-wrap items-center text-[13px]">
        {f &&
          folderCrumbs(f.path, f.home).map((c, i, all) =>
            i === all.length - 1 ? (
              <span key={c.path} className="px-1 font-medium text-tx">
                {c.name}
              </span>
            ) : (
              <span key={c.path} className="flex items-center">
                <button
                  type="button"
                  onClick={() => go(c.path)}
                  className="rounded px-1 py-0.5 text-tx3 hover:bg-hover hover:text-tx"
                >
                  {c.name}
                </button>
                <span className="text-tx4">/</span>
              </span>
            ),
          )}
      </nav>
      <div className="flex max-h-[min(280px,36vh)] flex-col overflow-y-auto rounded-[9px] border border-line2 bg-term">
        {browser.error && (
          <p className="m-0 px-3 py-2 text-[13px] text-err">{browser.error.message}</p>
        )}
        {f?.entries.map((e) =>
          // Foreman's own folders show, but don't open: no code repo goes there.
          e.foreman ? (
            <div
              key={e.path}
              title={`${e.foreman}: Foreman's own, not for code repos`}
              className="flex items-center gap-2 border-b border-line px-3 py-1.5 text-[13.5px] text-tx4 last:border-b-0"
            >
              <span className="text-accTx">
                <ForemanFolderIcon size="size-4" />
              </span>
              <span className="min-w-0 flex-1 truncate">{e.name}</span>
              <span className="shrink-0 text-[12px]">{e.foreman}</span>
            </div>
          ) : (
            <button
              key={e.path}
              type="button"
              onClick={() => go(e.path)}
              title={e.repo ? `${e.name}: a git repo` : e.name}
              className="flex items-center gap-2 border-b border-line px-3 py-1.5 text-left text-[13.5px] text-tx2 last:border-b-0 hover:bg-hover hover:text-tx"
            >
              <span className={e.repo ? "text-ok" : "text-tx4"}>
                {e.repo ? <RepoIcon size="size-4" /> : <FolderIcon open={false} size="size-4" />}
              </span>
              <span className="min-w-0 flex-1 truncate">{e.name}</span>
            </button>
          ),
        )}
        {f && f.entries.length === 0 && (
          <p className="m-0 px-3 py-2 text-[13px] text-tx4">No folders here.</p>
        )}
      </div>
      {failed && <p className="m-0 text-[13px] text-err">{failed}</p>}
    </div>
  );
}
