// screenshot-diff <a.png> <b.png> [diff.png]: counts the pixels that differ,
// with no tolerance, and draws them in red into diff.png when given. Exits 0
// when the images are the same, 1 when they differ, 2 on bad input.
import fs from "node:fs";
import pixelmatch from "pixelmatch";
import { PNG } from "pngjs";

const [a, b, out] = process.argv.slice(2);
if (!a || !b) {
  console.error("usage: screenshot-diff <a.png> <b.png> [diff.png]");
  process.exit(2);
}
function read(file) {
  try {
    return PNG.sync.read(fs.readFileSync(file));
  } catch (e) {
    console.error(`screenshot-diff: ${file}: ${e.message}`);
    process.exit(2);
  }
}
const [one, two] = [read(a), read(b)];
if (one.width !== two.width || one.height !== two.height) {
  console.log(`sizes differ: ${one.width}x${one.height} and ${two.width}x${two.height}`);
  process.exit(1);
}
const diff = out ? new PNG({ width: one.width, height: one.height }) : null;
const n = pixelmatch(one.data, two.data, diff?.data, one.width, one.height, { threshold: 0 });
if (diff) fs.writeFileSync(out, PNG.sync.write(diff));
console.log(n ? `${n} pixels differ` : "same");
process.exit(n ? 1 : 0);
