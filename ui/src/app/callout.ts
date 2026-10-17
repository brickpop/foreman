import type { Blockquote, Nodes, Root } from "mdast";

// Copied from wikiview v0.13.0 (ui/src/markdown/callout.ts), whose UI isn't a
// package Foreman can import (wiki/technical-debt.md).

/**
 * Turns a callout (`> [!warning] Title`) into a labelled blockquote. Any word
 * is a type, GitHub's and Obsidian's alike; a few get a colour in styles.css.
 * The marker is stripped and the type and label ride out as data attributes.
 * Nodes are mutated, never rebuilt: the renderer matches headings and
 * checkboxes by their source position, which a rebuilt node would lose.
 */
export function remarkCallout() {
  return (tree: Root) => {
    forEachBlockquote(tree, (quote) => {
      const paragraph = quote.children[0];
      if (paragraph?.type !== "paragraph") return;
      const text = paragraph.children[0];
      if (text?.type !== "text") return;

      const marker = MARKER.exec(text.value);
      if (!marker) return;

      text.value = text.value.slice(marker[0].length);
      if (!text.value) paragraph.children.shift();
      if (paragraph.children.length === 0) quote.children.shift();

      const type = marker[1].toLowerCase();
      const title = marker[2].trim();
      quote.data = {
        ...quote.data,
        hProperties: {
          ...quote.data?.hProperties,
          "data-callout": type,
          "data-callout-label": title || type.charAt(0).toUpperCase() + type.slice(1),
        },
      };
    });
  };
}

/** `[!type]`, an optional fold suffix (not implemented: folded text hides from ⌘F), an optional title. */
const MARKER = /^\[!([^\]\s]+)\][+-]?[ \t]*([^\n]*)(?:\n|$)/;

function forEachBlockquote(node: Nodes, visit: (quote: Blockquote) => void) {
  if (node.type === "blockquote") visit(node);
  if ("children" in node) for (const child of node.children) forEachBlockquote(child, visit);
}
