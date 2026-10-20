import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { useClient } from "../api/client";
import { keys, useProjects } from "../app/queries";
import { Modal } from "./Overlays";
import { useToast } from "./Toasts";
import { Button } from "./ui";

// Creates a project in a new folder, or turns an existing workspace folder
// into one (wiki/product/features/add-project.md). Foreman never clones: the
// CEO puts the repos in a folder first, or has Foreman create an empty one.

export function repoDetail(repos: string[]): string {
  if (repos.length === 0) return "No git repos yet";
  return `${repos.length} git repo${repos.length === 1 ? "" : "s"}: ${repos.join(", ")}`;
}

// nameClash says why a new project can't take name: a workspace folder has it
// already, in any case. The server refuses too, and also knows the hidden
// folders and plain files the dialog doesn't see.
export function nameClash(name: string, projects: string[], candidates: string[]): string {
  const lower = name.trim().toLowerCase();
  if (!lower) return "";
  const project = projects.find((p) => p.toLowerCase() === lower);
  if (project) return `${project} is already a project.`;
  const folder = candidates.find((c) => c.toLowerCase() === lower);
  if (folder) return `${folder}/ already exists: pick it below to add it.`;
  return "";
}

export function repoHint(folder: string, repo: string): string {
  return `Creates ${folder || "<project>"}/${repo || "<repo>"}/ and initializes a git repo in it, with a first commit. Leave it blank to add one later.`;
}

// Fields as in the Deploy dialog: a short label, the input, a hint under it.
const label = "text-[13px] font-medium text-tx4";
const field =
  "rounded-lg border border-line2 bg-term px-3 py-2 font-mono text-[13px] text-tx disabled:opacity-45";
const hint = "text-[12.5px] text-tx4";

export function AddProject({ open, onClose }: { open: boolean; onClose: () => void }) {
  const client = useClient();
  const queries = useQueryClient();
  const navigate = useNavigate();
  const toast = useToast();
  const projects = useProjects();
  const [name, setName] = useState("");
  const [chosen, setChosen] = useState<string>();
  // undefined follows the new project's name until the CEO edits it.
  const [repo, setRepo] = useState<string>();
  const candidates = useQuery({
    queryKey: ["candidates"],
    queryFn: () => client.workspace.candidates(),
    enabled: open,
  });
  const add = useMutation({
    mutationFn: ({ folder, repo }: { folder?: string; repo: string }) =>
      folder
        ? client.workspace.addProject(folder, repo || undefined)
        : client.workspace.newProject(name.trim(), repo || undefined),
    onSuccess: (p) => {
      queries.invalidateQueries({ queryKey: keys.projects });
      queries.invalidateQueries({ queryKey: ["candidates"] });
      onClose();
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
  const list = candidates.data ?? [];
  const selected = list.find((c) => c.folder === chosen);
  const folder = selected?.folder ?? name.trim();
  const repoValue = repo ?? (selected ? "" : name.trim());
  // The top Repo name field is the new folder's; a selected row has its own.
  const newRepo = selected ? "" : repoValue;
  const clash = selected
    ? ""
    : nameClash(
        name,
        (projects.data ?? []).map((p) => p.id),
        list.map((c) => c.folder),
      );
  return (
    <Modal open={open} onClose={onClose} title="New project">
      <div className="flex flex-col gap-4">
        <label className="flex flex-col gap-1.5">
          <span className={label}>Project name</span>
          {/* A project is named after its folder: a selected row shows that name,
              and editing it goes back to a new folder. */}
          <input
            value={selected?.folder ?? name}
            onChange={(e) => {
              setName(e.target.value);
              if (chosen) {
                setChosen(undefined);
                setRepo(undefined);
              }
            }}
            spellCheck={false}
            className={field}
          />
          {clash ? (
            <span className="text-[12.5px] text-err">{clash}</span>
          ) : selected ? (
            <span className={hint}>
              A project takes its folder’s name. Edit it to create a new folder instead.
            </span>
          ) : (
            <span className={hint}>
              Only letters, digits, dots, hyphens and underscores, starting with a letter or digit.
            </span>
          )}
        </label>
        <label className="flex flex-col gap-1.5">
          <span className={label}>Repo name</span>
          <input
            value={newRepo}
            onChange={(e) => setRepo(e.target.value)}
            disabled={!!selected}
            spellCheck={false}
            className={field}
          />
          <span className={hint}>{repoHint(name.trim(), newRepo.trim())}</span>
        </label>
        {list.length > 0 && (
          <fieldset className="m-0 flex flex-col gap-1.5 border-0 p-0">
            <legend className="mt-2 mb-1.5 text-[13px] font-medium text-tx4">
              or add an existing folder (Foreman never clones: put your repos in it first)
            </legend>
            {/* Its own scroll, so a workspace with many folders keeps the fields
                and the buttons in view; the padding leaves room for the focus ring. */}
            <div className="-m-0.5 flex max-h-[min(320px,40vh)] flex-col gap-1.5 overflow-y-auto p-0.5">
              {list.map((c) => {
                const on = c.folder === chosen;
                return (
                  <div
                    key={c.folder}
                    className={`flex flex-col gap-3 rounded-[9px] border px-3.5 py-3 has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-acc ${
                      on ? "border-acc bg-accSoft" : "border-line2 hover:border-line3"
                    }`}
                  >
                    <label className="flex cursor-pointer flex-col gap-0.5">
                      <input
                        type="radio"
                        name="folder"
                        value={c.folder}
                        checked={on}
                        onChange={() => {
                          setChosen(c.folder);
                          setName("");
                          setRepo(c.repos.length === 0 ? c.folder : "");
                        }}
                        className="sr-only"
                      />
                      <span className="font-mono text-sm font-medium">{c.folder}/</span>
                      <span className="text-[13px] text-tx3">{repoDetail(c.repos)}</span>
                    </label>
                    {on && (
                      <p className="m-0 text-[13px] text-tx2">
                        Creates {c.folder}/project/: a git repo with project.toml and a starter
                        wiki.
                      </p>
                    )}
                    {on && c.repos.length === 0 && (
                      <label className="flex flex-col gap-1.5">
                        <span className={label}>Repo name</span>
                        <input
                          value={repoValue}
                          onChange={(e) => setRepo(e.target.value)}
                          spellCheck={false}
                          className={field}
                        />
                        <span className={hint}>{repoHint(c.folder, repoValue.trim())}</span>
                      </label>
                    )}
                  </div>
                );
              })}
            </div>
          </fieldset>
        )}
        {add.error && <p className="m-0 text-[13px] text-err">{add.error.message}</p>}
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant="primary"
            disabled={!folder || !!clash || add.isPending}
            onClick={() => add.mutate({ folder: selected?.folder, repo: repoValue.trim() })}
          >
            {selected ? "Add project" : "Create project"}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
