import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { type CosItem, useClient } from "../api/client";
import { keys } from "../app/queries";
import { Drawer } from "./Overlays";

// The Chief of Staff drawer (wiki/product/features/chief-of-staff.md#drawer):
// its conversation, chips under the last message, and the input. Entry points
// start their flow through the client; this only shows and sends.

export function ChiefOfStaffDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const client = useClient();
  const queries = useQueryClient();
  const talk = useQuery({
    queryKey: keys.chiefOfStaff,
    queryFn: () => client.chiefOfStaff.conversation(),
    enabled: open,
  });
  const send = useMutation({
    mutationFn: (text: string) => client.chiefOfStaff.send(text),
    onSettled: () => queries.invalidateQueries({ queryKey: keys.chiefOfStaff }),
  });
  const [line, setLine] = useState("");
  const end = useRef<HTMLDivElement>(null);
  const items = talk.data?.items ?? [];
  const working = talk.data?.working ?? false;
  // Follow the conversation as it grows.
  // biome-ignore lint/correctness/useExhaustiveDependencies: scrolls when the conversation changes
  useEffect(() => end.current?.scrollIntoView({ block: "end" }), [items.length, working]);
  return (
    <Drawer
      open={open}
      onClose={onClose}
      width={420}
      layer={45}
      title={
        <span className="flex items-center gap-2.5">
          <span className="grid size-7 place-items-center rounded-[7px] bg-acc font-mono text-[13px] font-bold text-accInk">
            C
          </span>
          <span className="flex flex-col">
            <span>Chief of Staff</span>
            <span className="font-mono text-[11.5px] font-normal text-tx4">
              {talk.data?.session}
            </span>
          </span>
        </span>
      }
    >
      <div className="flex h-full flex-col">
        <div className="flex min-h-0 flex-1 flex-col gap-3.5 overflow-auto p-[18px]">
          {talk.error && <p className="m-0 text-[13px] text-err">{talk.error.message}</p>}
          {items.map((item) => (
            <Item key={item.id} item={item} />
          ))}
          {!working && (talk.data?.choices.length ?? 0) > 0 && (
            <div className="flex flex-wrap gap-2">
              {talk.data?.choices.map((c) => (
                <button
                  key={c}
                  type="button"
                  onClick={() => send.mutate(c)}
                  className="rounded-full border border-accLine px-3 py-1 text-[13px] text-accTx hover:bg-accSoft"
                >
                  {c}
                </button>
              ))}
            </div>
          )}
          {working && (
            <span className="text-[13px] text-tx4" role="status">
              Chief of Staff is working…
            </span>
          )}
          <div ref={end} />
        </div>
        <form
          className="border-t border-line p-3.5"
          onSubmit={(e) => {
            e.preventDefault();
            if (line.trim() === "") return;
            send.mutate(line);
            setLine("");
          }}
        >
          <input
            value={line}
            onChange={(e) => setLine(e.target.value)}
            placeholder="Ask for anything…"
            aria-label="Message the Chief of Staff"
            className="w-full rounded-[9px] border border-line2 bg-term px-3 py-2 text-tx placeholder:text-tx4 focus-visible:outline-2 focus-visible:outline-acc"
          />
          {send.error && <p className="m-0 pt-2 text-[13px] text-err">{send.error.message}</p>}
        </form>
      </div>
    </Drawer>
  );
}

function Item({ item }: { item: CosItem }) {
  if (item.panel) {
    return (
      <div className="flex flex-col gap-2 rounded-[10px] border border-line2 bg-term px-4 py-3.5">
        <span className="font-semibold">{item.panel.title}</span>
        <div className="grid grid-cols-[90px_1fr] gap-x-3 gap-y-1.5 text-[13.5px]">
          {item.panel.rows.map(([key, value], i) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: rows can repeat a key (several changes); their place is their identity
            <div key={i} className="contents">
              <span className="text-tx4">{key}</span>
              <span className="text-tx2">{value}</span>
            </div>
          ))}
        </div>
      </div>
    );
  }
  const ceo = item.from === "ceo";
  return (
    <p
      className={`m-0 max-w-[88%] rounded-xl px-[13px] py-2.5 leading-[1.5] whitespace-pre-wrap ${
        ceo ? "self-end border border-accLine bg-accSoft" : "self-start bg-raised"
      }`}
    >
      {item.text}
    </p>
  );
}
