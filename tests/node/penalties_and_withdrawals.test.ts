import test from "node:test";
import assert from "node:assert/strict";
import { amount, byId, runFixture } from "../helpers/ironclad.ts";

test("expired delivery is penalized and over-withdrawal is rejected", () => {
  const report = runFixture("penalty_withdrawal.json");
  const acme = byId(report.accounts, "acme");
  const treasury = byId(report.accounts, "treasury");
  const sub = byId(report.subs, "late-delivery:sub:0");
  const commitment = byId(report.commitments, "late-delivery");

  assert.equal(sub.state, "defaulted");
  assert.equal(sub.penalty, 5000);
  assert.equal(commitment.status, "closed");
  assert.equal(commitment.penalty_accrued, 5000);
  assert.equal(commitment.released_to_sponsor, 45000);
  assert.equal(report.rejected.length, 1);
  assert.equal(report.rejected[0]?.id, "too-large");
  assert.equal(report.rejected[0]?.code, "insufficient_funds");
  assert.equal(acme.external_out, 90000);
  assert.equal(amount(acme.reserves, "iusd"), 5000);
  assert.equal(amount(treasury.reserves, "iusd"), 5000);
  assert.equal(report.metrics.economic_shortfall, 0);
});
