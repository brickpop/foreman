import { codePieces } from "../app/codeText";

/** Prose with its `code` and fenced blocks shown as code; all of it as text,
 * never HTML. */
export function CodeText({ text }: { text: string }) {
  return codePieces(text).map((p, i) => {
    const key = `${i}${p.kind}`;
    if (p.kind === "code")
      return (
        <code
          key={key}
          className="rounded-[4px] bg-term px-1 py-px font-mono text-[0.92em] text-tx"
        >
          {p.text}
        </code>
      );
    if (p.kind === "block")
      return (
        <pre
          key={key}
          className="my-1 overflow-x-auto rounded-md border border-line bg-term px-2.5 py-2 font-mono text-[12px] leading-[1.5] whitespace-pre text-tx"
        >
          {p.text}
        </pre>
      );
    return <span key={key}>{p.text}</span>;
  });
}
