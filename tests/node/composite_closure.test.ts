import test from "node:test";
import assert from "node:assert/strict";
import { amount, byId, runFixture } from "../helpers/ironclad.ts";

test("composite commitment closes cleanly once every child is claimed", () => {
  const report = runFixture("composite_closure.json");
  const acme = byId(report.accounts, "acme");
  const carrierA = byId(report.accounts, "carrier-a");
  const carrierB = byId(report.accounts, "carrier-b");
  const commitment = byId(report.commitments, "bundle-healthy");
  const alpha = byId(report.subs, "bundle-healthy:a");
  const beta = byId(report.subs, "bundle-healthy:b");
  const parentLock = byId(report.locks, "bundle-healthy:parent");

  assert.equal(commitment.composite, true);
  assert.equal(commitment.status, "closed");
  assert.equal(commitment.claimed, 100000);
  assert.equal(commitment.released_to_sponsor, 0);
  assert.equal(alpha.state, "claimed");
  assert.equal(beta.state, "claimed");
  assert.equal(parentLock.state, "closed");
  assert.equal(amount(acme.reserves, "iusd"), 200000);
  assert.equal(amount(carrierA.reserves, "iusd"), 60000);
  assert.equal(amount(carrierB.reserves, "iusd"), 40000);
  assert.equal(report.metrics.economic_shortfall, 0);
});
