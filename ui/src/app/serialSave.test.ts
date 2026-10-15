import { describe, expect, it } from "vitest";
import { serialSaver } from "./serialSave";

/** A file on a server: a save is refused unless it sends the current version. */
function server(start: number) {
  const file = { value: start, version: "v0" };
  let n = 0;
  return {
    file,
    save: async (value: number, version: string) => {
      await Promise.resolve(); // a round trip
      if (version !== file.version) throw Object.assign(new Error("changed"), { code: "CONFLICT" });
      file.value = value;
      file.version = `v${++n}`;
      return { ...file };
    },
  };
}

describe("serialSaver", () => {
  it("applies quick changes in a row, each on top of the last save", async () => {
    const s = server(1);
    const saver = serialSaver(s.save);
    saver.seen({ ...s.file });
    const one = saver.change((v) => v + 1);
    const two = saver.change((v) => v + 1);
    await Promise.all([one, two]);
    expect(saver.lastSaved()).toBe("v2");
  });

  it("refuses a change made elsewhere, then saves again once it has seen it", async () => {
    const s = server(1);
    const saver = serialSaver(s.save);
    saver.seen({ ...s.file });
    await s.save(10, "v0"); // someone else
    await expect(saver.change((v) => v + 1)).rejects.toMatchObject({ code: "CONFLICT" });
    saver.seen({ ...s.file });
    await saver.change((v) => v + 1);
    expect(s.file.value).toBe(11);
  });

  it("ignores what the server serves while its own saves are on their way", async () => {
    const s = server(1);
    const saver = serialSaver(s.save);
    saver.seen({ ...s.file });
    const one = saver.change((v) => v + 1);
    saver.seen({ value: 1, version: "v0" }); // a refetch from before the save landed
    await one;
    await saver.change((v) => v + 1);
    expect(s.file).toEqual({ value: 3, version: "v2" });
  });

  it("says so when nothing was loaded", async () => {
    await expect(serialSaver(server(1).save).change((v) => v)).rejects.toThrow("Not loaded");
  });
});
