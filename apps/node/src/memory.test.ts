import { describe, expect, it } from "vitest";
import { MemoryStore } from "./memory.js";
import type { Comment, Resource, User } from "./models.js";
import { ErrNotFound } from "./store.js";

describe("MemoryStore", () => {
  it("seeds the fixture the contract declares", async () => {
    const s = new MemoryStore();
    await s.seed();
    const u = await s.getUser("u_ada");
    expect(u.email).toBe("ada@acme.test");
    expect(u.role).toBe("admin");
  });

  it("returns not-found for a cross-tenant resource", async () => {
    const s = new MemoryStore();
    await s.seed();
    // r_globex_k8s belongs to org_globex, not org_acme.
    await expect(s.getResource("org_acme", "r_globex_k8s")).rejects.toBe(ErrNotFound);
  });

  // The instant every probe row shares. Later than every seeded row, so
  // a descending order must put the probes first and an ascending one
  // last.
  const PROBE_AT = "2026-03-20T12:00:00Z";

  // The seeded rows alone prove nothing about ordering: every tied pair
  // already sits in id-ascending file order, and a Map iterates in
  // insertion order, so no sort at all reproduces the declared order.
  // Each probe pair below shares an instant and is inserted in reverse
  // id order, which only a real sort can put right.
  it("orders resources by created_at then id", async () => {
    const s = new MemoryStore();
    await s.seed();

    const resources = (s as unknown as { resources: Map<string, Resource> }).resources;
    for (const id of ["r_acme_zz", "r_acme_aa"]) {
      resources.set(id, {
        id,
        name: "tiebreak-probe",
        description: "",
        sensitivity: "internal",
        organization_id: "org_acme",
        registered_by: "u_ada",
        created_at: PROBE_AT,
      });
    }

    const rows = await s.listOrgResources("org_acme");
    expect(rows.map((r) => r.id)).toEqual([
      "r_acme_pg",
      "r_acme_s3",
      "r_acme_stripe",
      "r_acme_aa",
      "r_acme_zz",
    ]);
  });

  it("orders members by created_at then id", async () => {
    const s = new MemoryStore();
    await s.seed();

    const users = (s as unknown as { users: Map<string, User> }).users;
    for (const id of ["u_acme_zz", "u_acme_aa"]) {
      users.set(id, {
        id,
        name: "Tiebreak Probe",
        email: `${id}@acme.test`,
        role: "member",
        organization_id: "org_acme",
        created_at: PROBE_AT,
      });
    }

    const rows = await s.listOrgMembers("org_acme");
    expect(rows.map((u) => u.id)).toEqual(["u_ada", "u_linus", "u_acme_aa", "u_acme_zz"]);
  });

  it("orders comments by created_at then id", async () => {
    const s = new MemoryStore();
    await s.seed();

    const comments = (s as unknown as { comments: Map<string, Comment> }).comments;
    for (const id of ["c_acme_zz", "c_acme_aa"]) {
      comments.set(id, {
        id,
        request_id: "req_acme_1",
        organization_id: "org_acme",
        author_id: "u_linus",
        body: "tiebreak-probe",
        created_at: PROBE_AT,
      });
    }

    const rows = await s.listRequestComments("org_acme", "req_acme_1");
    expect(rows.map((c) => c.id)).toEqual([
      "c_acme_1",
      "c_acme_2",
      "c_acme_3",
      "c_acme_aa",
      "c_acme_zz",
    ]);
  });

  // The duplicate-email rule. Over HTTP a duplicate can only be minted
  // after the user it duplicates, so insertion order and created_at order
  // always agree and no conformance check can tell the sort from a plain
  // map walk. Planting an older duplicate behind the seeded user is what
  // makes the sort load-bearing: without it this returns u_ada. One
  // lookup per case is enough here, unlike apps/go: a Map iterates in
  // insertion order and Array.prototype.sort is stable, so dropping the
  // id tiebreak fails this every time rather than most of the time.
  it("resolves a duplicate email to the earliest user, not the first stored", async () => {
    const s = new MemoryStore();
    await s.seed();

    // Positive control: the lookup answers before any duplicate exists.
    expect((await s.findUserByEmail("ada@acme.test")).id).toBe("u_ada");

    const users = (s as unknown as { users: Map<string, User> }).users;
    const older: User = {
      id: "u_older",
      name: "Ada Elder",
      email: "ada@acme.test",
      role: "member",
      organization_id: "org_acme",
      created_at: "2020-01-01T00:00:00Z",
    };
    users.set(older.id, older);
    expect((await s.findUserByEmail("ada@acme.test")).id).toBe("u_older");

    // Tied created_at, stored last, so only the id tiebreak can win it.
    users.set("u_aaa", { ...older, id: "u_aaa", name: "Ada Tie" });
    expect((await s.findUserByEmail("ada@acme.test")).id).toBe("u_aaa");

    await expect(s.findUserByEmail("nobody@acme.test")).rejects.toBe(ErrNotFound);
  });

  // Revoking undoes the grant, not the record of who granted it. Until
  // task 7 this was written down only by req_globex_2's shape in the seed
  // fixture, which is seeded revoked rather than revoked at runtime.
  it("keeps decided_by and decided_at when a request is revoked", async () => {
    const s = new MemoryStore();
    await s.seed();

    const approved = await s.approveRequest("org_acme", "req_acme_1", "u_ada");
    expect(approved.decided_by).toBe("u_ada");
    expect(approved.decided_at).toBeTruthy();

    const revoked = await s.revokeRequest("org_acme", "req_acme_1", "u_ada");
    expect(revoked.status).toBe("revoked");
    expect(revoked.decided_by).toBe("u_ada");
    expect(revoked.decided_at).toBe(approved.decided_at);
    expect(revoked.revoked_by).toBe("u_ada");
    expect(revoked.revoked_at).toBeTruthy();
  });

  // Re-stamping, not just stamping: a store that returned the first
  // stamp unchanged passed while both assertions were toBeTruthy. The
  // wait is because this clock has millisecond resolution, so two calls
  // in the same millisecond would stamp the same instant honestly.
  it("decommissions idempotently, re-stamping decommission_at", async () => {
    const s = new MemoryStore();
    await s.seed();
    const first = await s.decommissionResource("org_acme", "r_acme_pg");
    expect(first.decommission_at).toBeTruthy();

    await new Promise((resolve) => setTimeout(resolve, 5));

    const second = await s.decommissionResource("org_acme", "r_acme_pg");
    expect(second.decommission_at).toBeTruthy();
    expect(second.decommission_at).not.toBe(first.decommission_at);
  });
});
