import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { isNotBuilt, useClient } from "../api/client";
import { keys, useWorkspace } from "../app/queries";
import { Modal } from "./Overlays";
import { useToast } from "./Toasts";
import { Button, NotBuilt } from "./ui";

// Turns a workspace folder into a project (wiki/product/features/add-project.md).
// Foreman never clones: the CEO puts the repos in the folder first.

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
  const candidates = useQuery({
    queryKey: ["candidates"],
    queryFn: () => client.workspace.candidates(),
    enabled: open,
  });
  const add = useMutation({
    mutationFn: (folder: string) => client.workspace.addProject(folder),
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
  if (isNotBuilt(candidates.error))
    return (
      <Modal open={open} onClose={onClose} title="Add a project">
        <NotBuilt what="adding projects" />
      </Modal>
    );
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
                  onChange={() => setChosen(c.folder)}
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
            {selected.repos.length === 0 && " Agents need at least one repo to work on."}
          </p>
        )}
        {add.error && <p className="m-0 text-[13px] text-err">{add.error.message}</p>}
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant="primary"
            disabled={!chosen || add.isPending}
            onClick={() => chosen && add.mutate(chosen)}
          >
            Add project
          </Button>
        </div>
      </div>
    </Modal>
  );
}
