import { NotBuilt } from "../components/ui";

// The Chief of Staff (wiki/product/features/chief-of-staff.md#the-page): its
// terminal, then the preview of the team it proposes. The server doesn't run
// it yet (wiki/backlog/cos-create-team.md).

export function ChiefOfStaff() {
  return (
    <div className="flex h-full flex-col">
      <header className="flex items-center gap-2.5 border-b border-line px-6 py-3.5">
        <span className="grid size-7 place-items-center rounded-[7px] bg-acc font-mono text-[13px] font-bold text-accInk">
          C
        </span>
        <h1 className="m-0 text-[15px] font-semibold">Chief of Staff</h1>
      </header>
      <div className="p-6">
        <NotBuilt what="the Chief of Staff" />
      </div>
    </div>
  );
}
