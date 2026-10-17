import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";

function git(...args) {
  const result = spawnSync("git", args, { encoding: "utf8" });
  if (result.status !== 0) throw new Error(result.stderr || `git ${args.join(" ")} failed`);
  return result.stdout.trim();
}

const version = JSON.parse(readFileSync("package.json", "utf8")).version;
if (version !== "1.0.0") throw new Error(`package version must be 1.0.0, got ${version}`);
const changelog = readFileSync("CHANGELOG.md", "utf8");
if (!changelog.includes("## [1.0.0]")) throw new Error("CHANGELOG does not contain 1.0.0");

const refType = process.env.GITHUB_REF_TYPE ?? "";
const refName = process.env.GITHUB_REF_NAME ?? "";
if (refType === "tag") {
  if (refName !== "v1.0.0") throw new Error(`unexpected release tag: ${refName}`);
  if (git("cat-file", "-t", `refs/tags/${refName}`) !== "tag") {
    throw new Error(`${refName} must be an annotated tag`);
  }
  const tagged = git("rev-list", "-n", "1", `refs/tags/${refName}`);
  for (const branch of ["origin/main", "origin/production"]) {
    if (git("rev-parse", branch) !== tagged) throw new Error(`${branch} does not match ${refName}`);
  }
}
console.log(`release metadata verified for ${refType || "local"}:${refName || "workspace"}`);
