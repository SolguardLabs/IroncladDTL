import test from "node:test";
import assert from "node:assert/strict";
import { IroncladClient } from "../../client/ironclad-client.ts";

test("client validates and summarizes a deterministic scenario", () => {
  const client = new IroncladClient();
  assert.equal(client.validate("tests/fixtures/base_lifecycle.json"), "ok base_lifecycle");
  const snapshot = client.simulate("tests/fixtures/base_lifecycle.json");
  const summary = client.summarize(snapshot);

  assert.deepEqual(summary, {
    scenario: "base_lifecycle",
    epoch: 0,
    vaults: 1,
    commitments: 1,
    openCommitments: 0,
    reserve: 0,
    locked: 0,
    openClaims: 0,
    rejectedOperations: 0,
  });
  assert.equal(Object.isFrozen(summary), true);
});

test("client can request the canonical event stream", () => {
  const client = new IroncladClient();
  const snapshot = client.simulate("tests/fixtures/composite_closure.json", { events: true });
  assert.equal(snapshot.commitments[0]?.status, "closed");
  assert.equal(snapshot.rejected.length, 0);
});
