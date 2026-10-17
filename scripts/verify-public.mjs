import { existsSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";

const required = [
  "README.md",
  "SECURITY.md",
  "CHANGELOG.md",
  "CONTRIBUTING.md",
  "assets/banner.png",
  "docs/arquitectura.md",
  "docs/modelo-economico.md",
  "docs/modelo-seguridad.md",
  "docs/operacion.md",
  "docs/integracion.md",
  "docs/gobernanza.md",
];

const listed = spawnSync(
  "git",
  ["ls-files", "-z", "--cached", "--others", "--exclude-standard"],
  { encoding: "utf8" },
);
if (listed.status !== 0) {
  console.error(listed.stderr);
  process.exit(listed.status ?? 1);
}
const files = listed.stdout.split("\0").filter((path) => path.length > 0 && existsSync(path));
const tracked = new Set(files);
for (const path of required) {
  if (!tracked.has(path)) {
    console.error(`required tracked file is missing: ${path}`);
    process.exit(1);
  }
}

const skip = new Set(["scripts/verify-public.mjs", "package-lock.json", "assets/banner.png"]);
const disallowed = [
  /\bctf\b/iu,
  /\blaboratorio(?:s)?\b/iu,
  /\bvulner(?:able|abilidad|ability|abilities)\b/iu,
  /\bexploit(?:s|ed|ing|acion|ación)?\b/iu,
  /\bbug(?:s|gy)?\b/iu,
  /\bfallo intencional\b/iu,
];
for (const path of files) {
  if (skip.has(path)) continue;
  const content = readFileSync(path, "utf8");
  for (const pattern of disallowed) {
    if (pattern.test(content)) {
      console.error(`disallowed public wording in ${path}: ${pattern}`);
      process.exit(1);
    }
  }
}
console.log(`public surface verified (${files.length} tracked files)`);
