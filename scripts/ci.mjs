import { spawnSync } from "node:child_process";

function run(command, args) {
  const result = spawnSync(command, args, {
    cwd: process.cwd(),
    encoding: "utf8",
    stdio: "inherit",
    shell: false,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}

function capture(command, args) {
  const result = spawnSync(command, args, {
    cwd: process.cwd(),
    encoding: "utf8",
    shell: false,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    process.stderr.write(result.stderr);
    process.exit(result.status ?? 1);
  }
  return result.stdout.trim();
}

const unformatted = capture("gofmt", ["-l", "src"]);
if (unformatted.length > 0) {
  console.error(unformatted);
  process.exit(1);
}

run("go", ["vet", "./..."]);
run("go", ["test", "./..."]);
run(process.execPath, ["scripts/build.mjs"]);
run(process.execPath, ["scripts/check-loc.mjs"]);
run(process.execPath, [
  "--test",
  "--experimental-strip-types",
  "tests/node/client.test.ts",
  "tests/node/composite_closure.test.ts",
  "tests/node/lock_lifecycle.test.ts",
  "tests/node/penalties_and_withdrawals.test.ts",
]);
run(process.execPath, ["scripts/verify-public.mjs"]);
run(process.execPath, ["scripts/verify-release.mjs"]);
