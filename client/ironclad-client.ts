import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export type ClientOptions = Readonly<{
  root?: string;
  binary?: string;
  timeoutMs?: number;
}>;

export type SimulationOptions = Readonly<{
  events?: boolean;
  compact?: boolean;
}>;

export type AmountEntry = Readonly<{ id: string; amount: number }>;

export type ProtocolSnapshot = Readonly<{
  name: string;
  epoch: number;
  accounts: ReadonlyArray<Readonly<{ id: string; reserves: ReadonlyArray<AmountEntry> }>>;
  vaults: ReadonlyArray<
    Readonly<{
      id: string;
      asset: string;
      reserve: number;
      locked: number;
      fees: number;
      open_claims: number;
    }>
  >;
  commitments: ReadonlyArray<
    Readonly<{
      id: string;
      amount: number;
      delivered: number;
      claimed: number;
      status: string;
      terminal: boolean;
    }>
  >;
  metrics: Readonly<Record<string, number>>;
  rejected: ReadonlyArray<Readonly<{ code: string; field?: string; type: string }>>;
}>;

export type PortfolioSummary = Readonly<{
  scenario: string;
  epoch: number;
  vaults: number;
  commitments: number;
  openCommitments: number;
  reserve: number;
  locked: number;
  openClaims: number;
  rejectedOperations: number;
}>;

const moduleRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");

export class IroncladClient {
  readonly root: string;
  readonly binary: string;
  readonly timeoutMs: number;

  constructor(options: ClientOptions = {}) {
    this.root = resolve(options.root ?? moduleRoot);
    this.binary = resolve(
      options.binary ??
        join(this.root, "build", process.platform === "win32" ? "ironcladdtl.exe" : "ironcladdtl"),
    );
    this.timeoutMs = options.timeoutMs ?? 15_000;
  }

  simulate(scenarioPath: string, options: SimulationOptions = {}): ProtocolSnapshot {
    const args = ["run", resolve(this.root, scenarioPath), "--json"];
    if (options.events) args.push("--events");
    if (options.compact) args.push("--compact");
    return this.executeJson(args) as ProtocolSnapshot;
  }

  validate(scenarioPath: string): string {
    return this.execute(["validate", resolve(this.root, scenarioPath)]).trim();
  }

  listScenarios(): ReadonlyArray<string> {
    return this.execute(["scenarios"])
      .split(/\r?\n/u)
      .map((line) => line.trim())
      .filter(Boolean);
  }

  summarize(snapshot: ProtocolSnapshot): PortfolioSummary {
    const reserve = snapshot.vaults.reduce((total, vault) => total + vault.reserve, 0);
    const locked = snapshot.vaults.reduce((total, vault) => total + vault.locked, 0);
    const openClaims = snapshot.vaults.reduce((total, vault) => total + vault.open_claims, 0);
    return Object.freeze({
      scenario: snapshot.name,
      epoch: snapshot.epoch,
      vaults: snapshot.vaults.length,
      commitments: snapshot.commitments.length,
      openCommitments: snapshot.commitments.filter((item) => !item.terminal).length,
      reserve,
      locked,
      openClaims,
      rejectedOperations: snapshot.rejected.length,
    });
  }

  private executeJson(args: string[]): unknown {
    const output = this.execute(args);
    try {
      return JSON.parse(output) as unknown;
    } catch (error) {
      throw new Error(`IroncladDTL returned invalid JSON: ${String(error)}`);
    }
  }

  private execute(args: string[]): string {
    if (!existsSync(this.binary)) {
      throw new Error(`IroncladDTL binary not found at ${this.binary}. Run npm run build first.`);
    }
    const result = spawnSync(this.binary, args, {
      cwd: this.root,
      encoding: "utf8",
      shell: false,
      timeout: this.timeoutMs,
      windowsHide: true,
    });
    if (result.error) {
      throw result.error;
    }
    if (result.status !== 0) {
      throw new Error(result.stderr.trim() || `IroncladDTL exited with status ${result.status}`);
    }
    return result.stdout;
  }
}
