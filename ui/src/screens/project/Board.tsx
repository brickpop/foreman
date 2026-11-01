import * as Menu from "@radix-ui/react-dropdown-menu";
import { useNavigate } from "@tanstack/react-router";
import { type DragEvent, useEffect, useRef, useState } from "react";
import { type Board as BoardData, type Card, isNotBuilt } from "../../api/client";
import { agentOf } from "../../app/model";
import {
  useBoards,
  useDeploymentOn,
  useEntry,
  useMissing,
  useMoveCard,
  useProjectRepo,
  useTeams,
  useTick,
} from "../../app/queries";
import { Drawer } from "../../components/Overlays";
import { useToast } from "../../components/Toasts";
import { Button, Loading, NotBuilt, Segmented } from "../../components/ui";
import { WikiArticle } from "../../components/WikiArticle";
import { ScaffoldNotice, WikiSetup } from "../../components/WikiSetup";
import { useProject } from "../Project";

// A kanban over wiki entries (wiki/product/features/board.md). A move rewrites
// one or two frontmatter lines, version-guarded.

// Every column 240–320px (the CEO, 2026-10-17): readable cards, never a
// stretched one on a wide screen; the board scrolls sideways when it must.
const columnTrack = "minmax(240px, 320px)";

// The side sheet's width. While it is open the board gets as much room on its
// right (the CEO, 2026-10-21), so any column can be scrolled out from under
// the sheet to sit beside it.
const sheetWidth = 460;

/**
 * How far to scroll the board right so an opened card sits beside the sheet,
 * 16px clear of it: 0 when it already does. Measured on the screen: the
 * card's right edge, and the window's width the sheet takes its share of.
 */
export function revealBy(cardRight: number, windowWidth: number): number {
  return Math.max(0, Math.ceil(cardRight - (windowWidth - sheetWidth - 16)));
}

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

/**
 * The lanes a column lists, columns first and lanes inside them (the CEO,
 * 2026-10-21): those holding its cards, in the board's lane order; while a
 * card is dragged, every lane, so each is a place to drop it.
 */
export function lanesShown(lanes: string[], cards: Card[], dragging: boolean): string[] {
  return dragging ? lanes : lanes.filter((l) => cards.some((c) => c.lane === l));
}

/**
 * A cell's outline while a card is dragged: every cell that takes it is
 * outlined, at least 48px tall, and the one under the pointer turns accent.
 * Being under the pointer never changes its size: a cell that shrank would
 * slip out from under the pointer, and flicker.
 */
export function cellLook(takesDrop: boolean, over: boolean): string {
  if (!takesDrop) return "border-transparent";
  return `min-h-12 border-dashed ${over ? "border-acc" : "border-line2"}`;
}

const priorityColor: Record<string, string> = {
  quick: "text-ok",
  high: "text-err",
  medium: "text-attnTx",
  low: "text-tx4",
};

export function Board() {
  const project = useProject().id;
  const navigate = useNavigate();
  const lacking = useMissing(project).data ?? [];
  const boards = useBoards(project);
  const [chosen, setChosen] = useState<string>();
  const [toml, setToml] = useState(false);
  const [sheet, setSheet] = useState<string>();
  const [dragging, setDragging] = useState(false);
  const scroller = useRef<HTMLDivElement>(null);
  const toast = useToast();
  const move = useMoveCard(project, (e) =>
    toast({ title: "Card not moved", body: e.message, color: "var(--err)" }),
  );
  const list = boards.data ?? [];
  const board = list.find((b) => b.id === chosen) ?? list[0];
  if (boards.isPending)
    return (
      <div className="px-6 pt-6">
        <Loading rows={5} />
      </div>
    );
  if (isNotBuilt(boards.error))
    return (
      <div className="px-6 pt-6">
        <NotBuilt what="the wiki" />
      </div>
    );
  // Boards are declared in wiki.toml: without it there are none to show.
  if (lacking.some((m) => m.required))
    return (
      <div className="px-6 pt-6">
        <WikiSetup project={project} missing={lacking} />
      </div>
    );
  if (boards.error) return <p className="px-6 pt-6 text-[13px] text-err">{boards.error.message}</p>;
  if (!board) return <p className="px-6 pt-6 text-tx3">No board is declared in wiki.toml.</p>;
  const { columns, lanes } = board;

  const open = (path: string, card: HTMLElement) => {
    setSheet(path);
    // Next frame: the room beside the sheet is laid out by then.
    requestAnimationFrame(() => {
      const by = revealBy(card.getBoundingClientRect().right, window.innerWidth);
      if (by > 0) scroller.current?.scrollBy({ left: by, behavior: "smooth" });
    });
  };

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
                to: "/projects/$project/wiki",
                params: { project },
                search: { entry: card.path },
              }),
          }),
      },
    );
  };

  return (
    <div className="flex flex-1 flex-col gap-4 px-6 pt-6 pb-12">
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
      {/* Columns reach down to the footer note, all as tall as the tallest (the CEO, 2026-10-21). */}
      <div ref={scroller} className="flex flex-1 flex-col overflow-x-auto">
        <div
          className="grid min-w-fit flex-1 gap-2"
          style={{
            gridTemplateColumns: `repeat(${columns.length}, ${columnTrack})`,
            // As wide again as the sheet, so the columns keep their width.
            width: sheet ? `calc(100% + ${sheetWidth}px)` : undefined,
            paddingRight: sheet ? sheetWidth : 0,
          }}
        >
          {columns.map((column) => {
            const cards = board.cards.filter((x) => x.column === column);
            const cell = (lane: string, label?: string) => (
              <Cell
                key={lane}
                label={label}
                cards={lanes.length === 0 ? cards : cards.filter((x) => x.lane === lane)}
                // A cell for no status (or no lane value) takes no drops: see moveOf.
                droppable={column !== "" && (lanes.length === 0 || lane !== "")}
                dragging={dragging}
                onDrop={(card) => drop(card, column, lane)}
                onDrag={setDragging}
                allCards={board.cards}
                menu={(card) => (
                  <MoveMenu board={board} card={card} onMove={(c, l) => drop(card, c, l)} />
                )}
                onOpen={open}
              />
            );
            return (
              <div key={column} className="flex flex-col gap-1">
                <div className="px-1 pb-1 text-[13px]">
                  <span className="font-medium text-tx3">{columnLabel(column)}</span>{" "}
                  <span className="text-tx4">{cards.length}</span>
                </div>
                <div className="flex min-h-[72px] flex-1 flex-col gap-3 rounded-[10px] border border-line bg-side p-2">
                  {lanes.length === 0
                    ? cell("")
                    : lanesShown(lanes, cards, dragging).map((lane) =>
                        cell(lane, laneLabel(lane, board.laneField)),
                      )}
                </div>
              </div>
            );
          })}
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

/** One lane of a column (or the whole column, on a board without lanes): its
 * cards, and where a card is dropped. */
function Cell({
  label,
  cards,
  allCards,
  droppable,
  dragging,
  onDrop,
  onDrag,
  menu,
  onOpen,
}: {
  label?: string;
  cards: Card[];
  allCards: Card[];
  droppable: boolean;
  dragging: boolean;
  onDrop: (card: Card) => void;
  onDrag: (dragging: boolean) => void;
  menu: (card: Card) => React.ReactNode;
  onOpen: (path: string, card: HTMLElement) => void;
}) {
  const [over, setOver] = useState(false);
  const teams = useTeams();
  const deployment = useDeploymentOn(useProject().id)?.id ?? "";
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
        onDrag(false);
        const card = allCards.find((c) => c.path === e.dataTransfer.getData("text/plain"));
        if (card) onDrop(card);
      }}
      // While a card is dragged, every cell that takes it shows its outline.
      className={`-m-1 flex flex-col gap-2 rounded-lg border p-1 ${cellLook(dragging && droppable, over)}`}
    >
      {label && <span className="px-1 text-xs text-tx4">{label}</span>}
      {cards.map((card) => {
        const assignee = card.assignee
          ? agentOf(teams.data ?? [], deployment, card.assignee)
          : undefined;
        return (
          // biome-ignore lint/a11y/noStaticElementInteractions: dragged and clicked; the keyboard paths are the title button and the status menu
          // biome-ignore lint/a11y/useKeyWithClickEvents: the title button is the keyboard path to the sheet
          <div
            key={card.path}
            // Clicking another card while the sheet is open shows that card in it.
            data-card
            // The whole card drags and opens (the CEO, 2026-10-21); only the
            // cursor tells them apart: a grab on its top row, a pointer below.
            draggable
            onClick={(e) => onOpen(card.path, e.currentTarget)}
            onDragStart={(e) => {
              e.dataTransfer.setData("text/plain", card.path);
              // After the drag has begun: changing the layout inside dragstart cancels it in Chrome.
              setTimeout(() => onDrag(true));
            }}
            onDragEnd={() => onDrag(false)}
            className="group flex cursor-pointer flex-col gap-1.5 rounded-lg border border-line bg-panel px-3 pb-2.5 hover:border-line3"
          >
            <div className="-mx-3 flex cursor-grab items-center gap-2 px-3 pt-2.5">
              <span className="font-mono text-[11.5px] font-medium text-tx4">{card.id}</span>
              {card.priority && (
                <span
                  className={`font-mono text-[11.5px] font-medium ${priorityColor[card.priority] ?? "text-tx4"}`}
                >
                  {card.priority}
                </span>
              )}
              <span className="flex-1" />
              {/* The menu moves the card; its clicks (in its portal too) don't open the sheet. */}
              {/* biome-ignore lint/a11y/noStaticElementInteractions: only stops the click from reaching the card */}
              {/* biome-ignore lint/a11y/useKeyWithClickEvents: keyboard events never open the sheet */}
              <span onClick={(e) => e.stopPropagation()}>{menu(card)}</span>
            </div>
            <button type="button" className="text-left text-sm leading-[1.4] text-tx">
              {card.title}
            </button>
            {assignee && <span className="text-[12.5px] text-tx3">{assignee.soul.name}</span>}
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
  // Another card shown in the open sheet starts at its top.
  const top = useRef<HTMLDivElement>(null);
  // biome-ignore lint/correctness/useExhaustiveDependencies: runs on each new card
  useEffect(() => {
    top.current?.parentElement?.scrollTo(0, 0);
  }, [path]);
  const open = (p: string) => {
    onClose();
    navigate({ to: "/projects/$project/wiki", params: { project }, search: { entry: p } });
  };
  return (
    <Drawer
      open={path !== undefined}
      onClose={onClose}
      width={sheetWidth}
      keepOpenOn="[data-card]"
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
      <div ref={top} className="px-[22px] pt-5 pb-8">
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
