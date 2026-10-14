import type { ReactNode } from "react";
import { Empty } from "../components/ui";

/** Where a screen will be: the card that builds it. Replaced by that card. */
export function ComingWith({ card, children }: { card: string; children?: ReactNode }) {
  return (
    <Empty>
      {children ?? "This screen"} comes with the <span className="text-tx2">{card}</span> card.
    </Empty>
  );
}
