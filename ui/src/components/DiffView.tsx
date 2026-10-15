import { useDiff } from "../app/queries";
import { Modal } from "./Overlays";

// What a merge request changes: its diff against main, one colored line per
// line (wiki/product/features/changes.md#review-and-merge).

const lineColor = (line: string) =>
  line.startsWith("+++") || line.startsWith("---")
    ? "text-tx4"
    : line.startsWith("+")
      ? "bg-ok/10 text-ok"
      : line.startsWith("-")
        ? "bg-err/10 text-err"
        : "text-tx3";

export function DiffView({
  mergeRequest,
  title,
  onClose,
}: {
  mergeRequest: string;
  title: string;
  onClose: () => void;
}) {
  const diff = useDiff(mergeRequest);
  return (
    <Modal open onClose={onClose} title={title} width={820}>
      <div className="max-h-[65vh] overflow-auto rounded-[10px] border border-line bg-term py-2 font-mono text-[12.5px] leading-[1.6] select-text">
        {diff.error && <p className="m-0 px-3.5 text-err">{diff.error.message}</p>}
        {diff.data?.split("\n").map((line, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: a diff line's place is its identity
          <div key={i} className={`px-3.5 whitespace-pre-wrap ${lineColor(line)}`}>
            {line || " "}
          </div>
        ))}
      </div>
    </Modal>
  );
}
