import * as Menu from "@radix-ui/react-dropdown-menu";
import { useNavigate } from "@tanstack/react-router";
import { type DragEvent, useState } from "react";
import { type Board as BoardData, type Card, isNotBuilt } from "../../api/client";
import { agentOf } from "../../app/model";
import {
  useBoards,
  useEntry,
  useMissing,
  useMoveCard,
  useProjectRepo,
  useTeams,
  useTick,
} from "../../app/queries";
import { Avatar } from "../../components/Avatar";
import { Drawer } from "../../components/Overlays";
import { useToast } from "../../components/Toasts";
import { Button, NotBuilt, Segmented } from "../../components/ui";
import { WikiArticle } from "../../components/WikiArticle";
import { ScaffoldNotice, WikiSetup } from "../../components/WikiSetup";

// A kanban over wiki entries (wiki/product/features/board.md). A move rewrites
// one or two frontmatter lines, version-guarded.

// Every column 240–320px (the CEO, 2026-10-17): readable cards, never a
// stretched one on a wide screen; the board scrolls sideways when it must.
const columnTrack = "minmax(240px, 320px)";

const columnLabels: Record<string, string> = {
  todo: "To do",
  "in-progress": "In progress",
  review: "Review",
  done: "Done",
};

/** A column's label; "" holds the cards with no status. */
export function columnLabel(value: string): string {
  if (value === "") return "No status";
  return columnLabels[value] ?? titleCase(value);
}

/** A lane's label; "" holds the cards without the lane field. */
export function laneLabel(value: string, field: string): string {
  return value === "" ? `No ${field}` : `${titleCase(value)} ${field}`;
}

function titleCase(s: string): string {
  const words = s.replaceAll("-", " ").replaceAll("_", " ");
  return words.charAt(0).toUpperCase() + words.slice(1);
}

/**
 * What dropping a card in a cell writes: its status, and its lane when that
 * changes (the server leaves an empty lane alone). Nothing when it stays put,
 * or when the cell is a "no value" one: writing there would remove the field,
 * which a move doesn't do. The columns and lanes come laid out from the server
 * (declared, then found, then "" for none), so no card is ever hidden.
 */
export function moveOf(
  board: BoardData,
  card: Card,
  column: string,
  lane: string,
): { column: string; lane: string } | undefined {
  const hasLanes = board.lanes.length > 0;
  if (column === "" || (hasLanes && lane === "" && card.lane !== "")) return undefined;
  if (column === card.column && (!hasLanes || lane === card.lane)) return undefined;
  return { column, lane: hasLanes && lane !== card.lane ? lane : "" };
}

const priorityColor: Record<string, string> = {
  quick: "text-ok",
  high: "text-err",
  medium: "text-accTx",
  low: "text-tx4",
};

export function Board({ project }: { project: string }) {
  const navigate = useNavigate();
  const lacking = useMissing(project).data ?? [];
  const boards = useBoards(project);
  const [chosen, setChosen] = useState<string>();
  const [toml, setToml] = useState(false);
  const [sheet, setSheet] = useState<string>();
  const toast = useToast();
  const move = useMoveCard(project, (e) =>
    toast({ title: "Card not moved", body: e.message, color: "var(--err)" }),
  );
  const list = boards.data ?? [];
  const board = list.find((b) => b.id === chosen) ?? list[0];
  if (boards.isPending) return null;
  if (isNotBuilt(boards.error))
    return (
      <div className="px-12 pt-6">
        <NotBuilt what="the wiki" />
      </div>
    );
  // Boards are declared in wiki.toml: without it there are none to show.
  if (lacking.some((m) => m.required))
    return (
      <div className="px-12 pt-6">
        <WikiSetup project={project} missing={lacking} />
      </div>
    );
  if (boards.error)
    return <p className="px-12 pt-6 text-[13px] text-err">{boards.error.message}</p>;
  if (!board) return <p className="px-12 pt-6 text-tx3">No board is declared in wiki.toml.</p>;
  const { columns, lanes } = board;

  const drop = (card: Card, column: string, lane: string) => {
    const to = moveOf(board, card, column, lane);
    if (!to) return;
    move.mutate(
      { board: board.id, path: card.path, column: to.column, lane: to.lane, version: card.version },
      {
        onSuccess: () =>
          toast({
            title: card.path,
            body: `${board.statusField}: ${to.column}${to.lane ? ` · ${board.laneField}: ${to.lane}` : ""}`,
            color: "var(--info)",
            onOpen: () =>
              navigate({
                to: "/projects/$project",
                params: { project },
                search: { tab: "overview", entry: card.path },
              }),
          }),
      },
    );
  };

  return (
    <div className="flex flex-col gap-4 px-12 pt-6 pb-16">
      {lacking.length > 0 && <ScaffoldNotice project={project} missing={lacking} />}
      <div className="flex items-center gap-3">
        {list.length > 1 && (
          <Segmented
            label="Boards"
            value={board.id}
            onChange={setChosen}
            options={list.map((b) => ({ value: b.id, label: b.name }))}
          />
        )}
        <span className="flex-1" />
        <Button variant="muted" className="font-mono" onClick={() => setToml(!toml)}>
          {toml ? "Hide wiki.toml" : "wiki.toml"}
        </Button>
      </div>
      {toml && (
        <pre className="max-w-[620px] rounded-[10px] border border-line bg-term p-3.5 font-mono text-[12.5px] leading-[1.6] text-tx2">
          {board.declaration}
        </pre>
      )}
      {board.problems.length > 0 && (
        <p className="text-[13px] text-err">{board.problems.join(" · ")}</p>
      )}
      <div className="overflow-x-auto">
        <div className="min-w-fit">
          <div
            className="grid gap-2"
            style={{ gridTemplateColumns: `repeat(${columns.length}, ${columnTrack})` }}
          >
            {columns.map((c) => (
              <div key={c} className="px-1 pb-1 text-[13px]">
                <span className="font-medium text-tx3">{columnLabel(c)}</span>{" "}
                <span className="text-tx4">{board.cards.filter((x) => x.column === c).length}</span>
              </div>
            ))}
          </div>
          {(lanes.length > 0 ? lanes : [""]).map((lane) => (
            <div key={lane} className="mt-3 flex flex-col gap-1.5">
              {lanes.length > 0 && (
                <span className="text-xs text-tx4">{laneLabel(lane, board.laneField)}</span>
              )}
              <div
                className="grid gap-2"
                style={{ gridTemplateColumns: `repeat(${columns.length}, ${columnTrack})` }}
              >
                {columns.map((column) => (
                  <Cell
                    key={column}
                    cards={board.cards.filter((x) => x.column === column && x.lane === lane)}
                    // A cell for no status (or no lane value) takes no drops: see moveOf.
                    droppable={column !== "" && (lanes.length === 0 || lane !== "")}
                    onDrop={(card) => drop(card, column, lane)}
                    allCards={board.cards}
                    menu={(card) => (
                      <MoveMenu board={board} card={card} onMove={(c, l) => drop(card, c, l)} />
                    )}
                    onOpen={setSheet}
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
      <p className="text-[12.5px] text-tx4">
        Drag a card to change its status (and its lane). Only those lines of the task's frontmatter
        are rewritten.
      </p>
      <SideSheet project={project} path={sheet} onClose={() => setSheet(undefined)} />
    </div>
  );
}

function Cell({
  cards,
  allCards,
  droppable,
  onDrop,
  menu,
  onOpen,
}: {
  cards: Card[];
  allCards: Card[];
  droppable: boolean;
  onDrop: (card: Card) => void;
  menu: (card: Card) => React.ReactNode;
  onOpen: (path: string) => void;
}) {
  const [over, setOver] = useState(false);
  const teams = useTeams();
  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: a drop target; the keyboard path is each card's status menu
    <div
      // Not cancelling dragover is how a cell says it takes no drop.
      onDragOver={(e: DragEvent) => {
        if (!droppable) return;
        e.preventDefault();
        setOver(true);
      }}
      // Moving over a card inside the cell is not leaving it.
      onDragLeave={(e: DragEvent) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setOver(false);
      }}
      onDrop={(e: DragEvent) => {
        e.preventDefault();
        setOver(false);
        const card = allCards.find((c) => c.path === e.dataTransfer.getData("text/plain"));
        if (card) onDrop(card);
      }}
      className={`flex min-h-[72px] flex-col gap-2 rounded-[10px] border bg-side p-2 ${over ? "border-acc" : "border-line"}`}
    >
      {cards.map((card) => {
        const assignee = card.assignee ? agentOf(teams.data ?? [], `${card.assignee}@`) : undefined;
        return (
          // biome-ignore lint/a11y/noStaticElementInteractions: dragged and clicked; the keyboard paths are the title button and the status menu
          // biome-ignore lint/a11y/useKeyWithClickEvents: the title button is the keyboard path to the sheet
          <div
            key={card.path}
            draggable
            // A click anywhere on the card opens its sheet (the title button's click lands here too).
            onClick={() => onOpen(card.path)}
            onDragStart={(e) => e.dataTransfer.setData("text/plain", card.path)}
            className="group flex cursor-grab flex-col gap-1.5 rounded-lg border border-line bg-panel px-3 py-2.5 hover:border-line3"
          >
            <div className="flex items-center gap-2">
              <span className="font-mono text-[11.5px] font-medium text-tx4">{card.id}</span>
              {card.priority && (
                <span
                  className={`font-mono text-[11.5px] font-medium ${priorityColor[card.priority] ?? "text-tx4"}`}
                >
                  {card.priority}
                </span>
              )}
              <span className="flex-1" />
              {/* The menu moves the card; its clicks don't open the sheet. */}
              {/* biome-ignore lint/a11y/noStaticElementInteractions: only stops the click from reaching the card */}
              {/* biome-ignore lint/a11y/useKeyWithClickEvents: keyboard events never open the sheet */}
              <span onClick={(e) => e.stopPropagation()}>{menu(card)}</span>
            </div>
            <button type="button" className="text-left text-sm leading-[1.4] text-tx">
              {card.title}
            </button>
            {assignee && (
              <span className="flex items-center gap-1.5 text-[12.5px] text-tx3">
                <Avatar look={assignee.soul.look} size={20} />
                {assignee.soul.name}
              </span>
            )}
          </div>
        );
      })}
    </div>
  );
}

/** The keyboard and touch way to move a card: the same as a drag. */
function MoveMenu({
  board,
  card,
  onMove,
}: {
  board: BoardData;
  card: Card;
  onMove: (column: string, lane: string) => void;
}) {
  const item =
    "cursor-pointer rounded px-2 py-1 text-[13px] text-tx2 outline-none data-[highlighted]:bg-hover";
  return (
    <Menu.Root>
      <Menu.Trigger aria-label={`Move ${card.id}`} className="rounded px-1 text-tx4 hover:text-tx2">
        ⋯
      </Menu.Trigger>
      <Menu.Portal>
        <Menu.Content className="z-50 flex min-w-40 flex-col rounded-lg border border-line2 bg-panel p-1 shadow-overlay">
          <Menu.Label className="px-2 py-1 text-xs text-tx4">
            {titleCase(board.statusField)}
          </Menu.Label>
          {/* "" (no value) isn't a value to move to: see moveOf. */}
          {board.columns
            .filter((c) => c !== "")
            .map((c) => (
              <Menu.Item
                key={c}
                className={item}
                // Only the status changes: the lane keeps its value, even one the board doesn't declare.
                onSelect={() => onMove(c, card.lane)}
              >
                {columnLabel(c)}
              </Menu.Item>
            ))}
          {board.lanes.length > 0 && (
            <>
              <Menu.Label className="px-2 py-1 text-xs text-tx4">
                {titleCase(board.laneField)}
              </Menu.Label>
              {board.lanes
                .filter((l) => l !== "")
                .map((l) => (
                  <Menu.Item key={l} className={item} onSelect={() => onMove(card.column, l)}>
                    {titleCase(l)}
                  </Menu.Item>
                ))}
            </>
          )}
        </Menu.Content>
      </Menu.Portal>
    </Menu.Root>
  );
}

/** A card's entry, beside the board (wiki/product/features/board.md#side-sheet). */
function SideSheet({
  project,
  path,
  onClose,
}: {
  project: string;
  path?: string;
  onClose: () => void;
}) {
  const repo = useProjectRepo(project);
  const navigate = useNavigate();
  const toast = useToast();
  const entry = useEntry(project, path ?? "");
  const tick = useTick(project, (e) =>
    toast({ title: path ?? "", body: e.message, color: "var(--err)" }),
  );
  const open = (p: string) => {
    onClose();
    navigate({
      to: "/projects/$project",
      params: { project },
      search: { tab: "overview", entry: p },
    });
  };
  return (
    <Drawer
      open={path !== undefined}
      onClose={onClose}
      width={460}
      title={
        <span className="flex items-center gap-3">
          <span className="font-mono text-[12.5px] font-normal text-tx3">
            {repo}/wiki/{path}
          </span>
          <Button variant="small" onClick={() => path && open(path)}>
            Open in wiki
          </Button>
        </span>
      }
    >
      <div className="px-[22px] pt-5 pb-8">
        {entry.data && (
          <WikiArticle
            project={project}
            entry={entry.data}
            size="sheet"
            onOpen={open}
            onTick={(line, done) =>
              path && tick.mutate({ path, line, done, version: entry.data.version })
            }
          />
        )}
      </div>
    </Drawer>
  );
}
