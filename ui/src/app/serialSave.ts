// Saves a version-guarded file one change at a time. Each change applies to
// what the previous save wrote and sends the version it returned, so quick
// changes in a row (two clicks on +, a swatch then Shuffle) don't refuse each
// other as conflicts. Only a change made elsewhere is a conflict.

export type Saved<T> = { value: T; version: string };

export function serialSaver<T>(save: (value: T, version: string) => Promise<Saved<T>>) {
  let latest: Saved<T> | undefined;
  let queue: Promise<unknown> = Promise.resolve();
  let pending = 0;
  let lastSaved: string | undefined;
  return {
    /** The version the last save here produced (a newer one came from elsewhere). */
    lastSaved: () => lastSaved,
    /** What the server last served; ignored while our own saves are on their way. */
    seen(file: Saved<T> | undefined) {
      if (pending === 0) latest = file;
    },
    /** Queues a change; resolves with what it saved. */
    change(apply: (value: T) => T): Promise<Saved<T>> {
      pending++;
      const run = queue
        .then(async () => {
          if (!latest) throw new Error("Not loaded yet");
          latest = await save(apply(structuredClone(latest.value)), latest.version);
          lastSaved = latest.version;
          return latest;
        })
        .finally(() => pending--);
      queue = run.catch(() => {}); // a failure doesn't stop the changes after it
      return run;
    },
  };
}

export type SerialSaver<T> = ReturnType<typeof serialSaver<T>>;
