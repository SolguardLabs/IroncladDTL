import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export type AmountEntry = {
  id: string;
  amount: number;
};

export type AccountReport = {
  id: string;
  label: string;
  reserves: AmountEntry[];
  external_in: number;
  external_out: number;
  claims_received: number;
  sponsor_releases: number;
  penalties_paid: number;
  rejected_attempts: number;
};

export type VaultReport = {
  id: string;
  asset: string;
  reserve: number;
  locked: number;
  fees: number;
  insolvency: number;
  claim_shortfall: number;
  open_claims: number;
};

export type CommitmentReport = {
  id: string;
  vault: string;
  sponsor: string;
  amount: number;
  delivered: number;
  claimed: number;
  released_to_sponsor: number;
  penalty_accrued: number;
  status: string;
  composite: boolean;
  release_threshold_bps: number;
  parent_lock_id: string;
  outstanding: number;
  terminal: boolean;
};

export type SubReport = {
  id: string;
  parent_id: string;
  beneficiary: string;
  amount: number;
  delivered: number;
  claimed: number;
  penalty: number;
  state: string;
  remaining: number;
  delivered_bps: number;
};

export type LockReport = {
  id: string;
  commitment: string;
  sub?: string;
  scope: string;
  amount: number;
  released: number;
  claimed: number;
  state: string;
};

export type IroncladReport = {
  name: string;
  epoch: number;
  accounts: AccountReport[];
  vaults: VaultReport[];
  commitments: CommitmentReport[];
  subs: SubReport[];
  locks: LockReport[];
  metrics: Record<string, number>;
  risk: {
    total_claims: number;
    total_reserve: number;
    total_shortfall: number;
    released_mismatch: number;
  };
  audit: {
    checks: Array<{ name: string; ok: boolean; details?: string }>;
    warnings: string[];
    has_critical_finding: boolean;
  };
  rejected: Array<{ type: string; id?: string; code: string; field?: string; message: string }>;
  events?: Array<Record<string, string | number | Record<string, string>>>;
};

export const root = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
export const binary = join(root, "build", process.platform === "win32" ? "ironcladdtl.exe" : "ironcladdtl");

export function ensureBuilt(): void {
  if (existsSync(binary)) {
    return;
  }
  const result = spawnSync(process.execPath, ["scripts/build.mjs"], {
    cwd: root,
    encoding: "utf8",
  });
  if (result.status !== 0) {
    throw new Error(result.stderr || result.stdout || "build failed");
  }
}

export function runCli(args: string[]): string {
  ensureBuilt();
  const result = spawnSync(binary, args, {
    cwd: root,
    encoding: "utf8",
  });
  if (result.status !== 0) {
    throw new Error(`command failed: ${binary} ${args.join(" ")}\n${result.stderr}\n${result.stdout}`);
  }
  return result.stdout;
}

export function runFixture(name: string, extra: string[] = []): IroncladReport {
  const fixture = join("tests", "fixtures", name);
  return JSON.parse(runCli(["run", fixture, "--json", ...extra])) as IroncladReport;
}

export function validateFixture(name: string): string {
  const fixture = join("tests", "fixtures", name);
  return runCli(["validate", fixture]).trim();
}

export function byId<T extends { id: string }>(items: T[], id: string): T {
  const found = items.find((item) => item.id === id);
  if (!found) {
    throw new Error(`missing id ${id}`);
  }
  return found;
}

export function amount(entries: AmountEntry[], id: string): number {
  return byId(entries, id).amount;
}

export function check(report: IroncladReport, name: string): boolean {
  const found = report.audit.checks.find((item) => item.name === name);
  if (!found) {
    throw new Error(`missing check ${name}`);
  }
  return found.ok;
}
