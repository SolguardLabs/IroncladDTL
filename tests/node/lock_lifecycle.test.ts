import test from "node:test";
import assert from "node:assert/strict";
import { amount, byId, check, runFixture, validateFixture } from "../helpers/ironclad.ts";

test("single-beneficiary lock moves from locked to claimable to closed", () => {
  assert.equal(validateFixture("base_lifecycle.json"), "ok base_lifecycle");

  const report = runFixture("base_lifecycle.json");
  const acme = byId(report.accounts, "acme");
  const carrier = byId(report.accounts, "carrier-a");
  const vault = byId(report.vaults, "iron-usd");
  const commitment = byId(report.commitments, "delivery-1");
  const sub = byId(report.subs, "delivery-1:sub:0");
  const parentLock = byId(report.locks, "delivery-1:parent");

  assert.equal(commitment.status, "closed");
  assert.equal(commitment.terminal, true);
  assert.equal(commitment.claimed, 50000);
  assert.equal(sub.state, "claimed");
  assert.equal(sub.claimed, 50000);
  assert.equal(parentLock.state, "closed");
  assert.equal(vault.reserve, 0);
  assert.equal(vault.locked, 0);
  assert.equal(amount(acme.reserves, "iusd"), 150000);
  assert.equal(amount(carrier.reserves, "iusd"), 50000);
  assert.equal(check(report, "economic_claims_covered"), true);
});
