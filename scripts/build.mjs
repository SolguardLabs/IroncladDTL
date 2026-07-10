import { existsSync, mkdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const outDir = join(root, "build");
const exeName = process.platform === "win32" ? "ironcladdtl.exe" : "ironcladdtl";
const output = join(outDir, exeName);

function run(command, args, options = {}) {
  return spawnSync(command, args, {
    cwd: root,
    encoding: "utf8",
    stdio: options.stdio ?? "pipe",
    shell: false,
  });
}

function commandExists(command) {
  const result =
    process.platform === "win32"
      ? run("where.exe", [command])
      : spawnSync("sh", ["-c", `command -v "${command.replaceAll('"', '\\"')}"`], {
          cwd: root,
          encoding: "utf8",
          stdio: "pipe",
        });
  return result.status === 0;
}

if (!commandExists("go")) {
  console.error("Go toolchain not found. Install Go 1.22+ or put go in PATH.");
  process.exit(1);
}

mkdirSync(outDir, { recursive: true });

const args = ["build", "-trimpath", "-o", output, "./src"];
if (process.argv.includes("--race")) {
  args.splice(1, 0, "-race");
}
const result = run("go", args, { stdio: "inherit" });
if (result.status !== 0) {
  process.exit(result.status ?? 1);
}

if (!existsSync(output)) {
  console.error(`Build completed but ${output} was not produced.`);
  process.exit(1);
}

console.log(output);
