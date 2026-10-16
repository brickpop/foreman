import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { useClient } from "../api/client";
import { keys, useWorkspace } from "../app/queries";
import { Modal } from "./Overlays";
import { useToast } from "./Toasts";
import { Button } from "./ui";

// Turns a workspace folder into a project (wiki/product/features/add-project.md).
// Foreman never clones: the CEO puts the repos in the folder first, or has
// Foreman create an empty one for a folder that has none.

export function repoDetail(repos: string[]): string {
  if (repos.length === 0) return "No git repos yet";
  return `${repos.length} git repo${repos.length === 1 ? "" : "s"}: ${repos.join(", ")}`;
}

export function AddProject({ open, onClose }: { open: boolean; onClose: () => void }) {
  const client = useClient();
  const queries = useQueryClient();
  const navigate = useNavigate();
  const toast = useToast();
  const workspace = useWorkspace();
  const [chosen, setChosen] = useState<string>();
  const [repo, setRepo] = useState("");
  const candidates = useQuery({
    queryKey: ["candidates"],
    queryFn: () => client.workspace.candidates(),
    enabled: open,
  });
  const add = useMutation({
    mutationFn: ({ folder, repo }: { folder: string; repo: string }) =>
      client.workspace.addProject(folder, repo.trim() || undefined),
    onSuccess: (p) => {
      queries.invalidateQueries({ queryKey: keys.projects });
      queries.invalidateQueries({ queryKey: ["candidates"] });
      onClose();
      setChosen(undefined);
      navigate({
        to: "/projects/$project",
        params: { project: p.id },
        search: { tab: "overview" },
      });
      toast({
        title: `${p.id} added`,
        body: "Created project/ with project.toml and a wiki. Deploy a team from its Settings tab.",
        color: "var(--ok)",
        onOpen: () =>
          navigate({
            to: "/projects/$project",
            params: { project: p.id },
            search: { tab: "settings" },
          }),
      });
    },
  });
  const root = workspace.data?.path ?? "the workspace";
  const list = candidates.data ?? [];
  const selected = list.find((c) => c.folder === chosen);
  return (
    <Modal open={open} onClose={onClose} title="Add a project">
      <div className="flex flex-col gap-4">
        <p className="m-0 text-[13.5px] text-tx3">
          Folders in {root} that aren’t projects yet. Clone your repos into a folder, then add it
          here.
        </p>
        {list.length === 0 ? (
          <p className="m-0 text-tx3">No new folders found in {root}.</p>
        ) : (
          <fieldset className="m-0 flex flex-col gap-1.5 border-0 p-0">
            <legend className="sr-only">Folders</legend>
            {list.map((c) => (
              <label
                key={c.folder}
                className={`flex cursor-pointer flex-col gap-0.5 rounded-[9px] border px-3.5 py-3 has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-acc ${
                  c.folder === chosen ? "border-acc bg-accSoft" : "border-line2 hover:border-line3"
                }`}
              >
                <input
                  type="radio"
                  name="folder"
                  value={c.folder}
                  checked={c.folder === chosen}
                  onChange={() => {
                    setChosen(c.folder);
                    setRepo(c.repos.length === 0 ? c.folder : "");
                  }}
                  className="sr-only"
                />
                <span className="font-mono text-sm font-medium">{c.folder}/</span>
                <span className="text-[13px] text-tx3">{repoDetail(c.repos)}</span>
              </label>
            ))}
          </fieldset>
        )}
        {selected && (
          <p className="m-0 text-[13px] text-tx2">
            Creates {selected.folder}/project/: a git repo with project.toml and a starter wiki.
          </p>
        )}
        {selected && selected.repos.length === 0 && (
          <label className="flex flex-col gap-1 text-[13px] text-tx3">
            New repo (agents need one to work on; Foreman creates it empty, with a first commit.
            Leave it blank to add one later.)
            <input
              value={repo}
              onChange={(e) => setRepo(e.target.value)}
              spellCheck={false}
              className="rounded-[7px] border border-line2 bg-transparent px-2.5 py-1.5 font-mono text-[13px] text-tx"
            />
          </label>
        )}
        {add.error && <p className="m-0 text-[13px] text-err">{add.error.message}</p>}
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant="primary"
            disabled={!chosen || add.isPending}
            onClick={() => chosen && add.mutate({ folder: chosen, repo })}
          >
            Add project
          </Button>
        </div>
      </div>
    </Modal>
  );
}
