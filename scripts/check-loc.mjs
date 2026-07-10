import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(fileURLToPath(new URL("..", import.meta.url)));
const src = join(root, "src");

function files(dir) {
  return readdirSync(dir)
    .flatMap((name) => {
      const path = join(dir, name);
      const stat = statSync(path);
      if (stat.isDirectory()) {
        return files(path);
      }
      return path.endsWith(".go") ? [path] : [];
    })
    .sort();
}

let lines = 0;
for (const file of files(src)) {
  const text = readFileSync(file, "utf8");
  lines += text.length === 0 ? 0 : text.split(/\r?\n/).length;
}

console.log(`src LOC: ${lines}`);
if (lines < 3000 || lines > 4000) {
  console.error("Expected src/ to stay in the requested 3000-4000 LOC range.");
  process.exit(1);
}
