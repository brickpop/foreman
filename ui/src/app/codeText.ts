// Code in agents' messages: the one piece of Markdown they lean on. Inline
// `code` and ``` fenced blocks ``` are told apart from prose; everything else
// stays plain text. A full Markdown renderer would be a second one beside the
// server's (the wiki): not worth it for messages.

export type Piece = { kind: "text" | "code" | "block"; text: string };

/** Text as prose, inline code and fenced blocks, in order. An unclosed fence
 * or backtick stays text. */
export function codePieces(text: string): Piece[] {
  const pieces: Piece[] = [];
  const blocks = /```[^\n]*\n([\s\S]*?)```/g;
  let at = 0;
  for (const m of text.matchAll(blocks)) {
    inline(text.slice(at, m.index), pieces);
    pieces.push({ kind: "block", text: m[1].replace(/\n$/, "") });
    at = m.index + m[0].length;
  }
  inline(text.slice(at), pieces);
  return pieces;
}

function inline(text: string, pieces: Piece[]) {
  let at = 0;
  for (const m of text.matchAll(/`([^`\n]+)`/g)) {
    if (m.index > at) pieces.push({ kind: "text", text: text.slice(at, m.index) });
    pieces.push({ kind: "code", text: m[1] });
    at = m.index + m[0].length;
  }
  if (at < text.length) pieces.push({ kind: "text", text: text.slice(at) });
}
