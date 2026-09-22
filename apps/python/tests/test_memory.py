import json
from pathlib import Path

import pytest

from app.memory import MemoryStore
from app.models import now_rfc3339, parse_rfc3339
from app.store import NotFoundError

PROBE_AT = "2026-03-20T12:00:00Z"

# A test may read the fixture. The rule that the app never reads it at runtime
# is what makes this comparison worth making.
CONTRACT_SEED = Path(__file__).resolve().parents[3] / "contract" / "seed.json"


def seeded():
    s = MemoryStore()
    s.seed()
    return s


def every_row(s):
    """Every seeded row, reached through the store's own scoped accessors."""
    rows = {"organizations": [], "users": [], "resources": [], "access_requests": [], "comments": []}
    for org in s.list_organizations():
        rows["organizations"].append(org)
        rows["users"] += s.list_org_members(org.id)
        for resource in s.list_org_resources(org.id):
            rows["resources"].append(resource)
            for request in s.list_resource_requests(org.id, resource.id):
                rows["access_requests"].append(request)
                rows["comments"] += s.list_request_comments(org.id, request.id)
    return {name: {r.id: r.to_dict() for r in group} for name, group in rows.items()}


def test_seeds_the_fixture_the_contract_declares():
    # Every collection, every row, every field, and field presence too: the
    # dicts compare by key set, so an optional field that should be absent
    # cannot pass by being present and null.
    expected = json.loads(CONTRACT_SEED.read_text())
    actual = every_row(seeded())
    for name in ("organizations", "users", "resources", "access_requests", "comments"):
        want = {row["id"]: row for row in expected[name]}
        assert actual[name] == want, name
    assert sum(len(group) for group in actual.values()) == 22


def test_cross_tenant_resource_is_not_found():
    s = seeded()
    # A same-tenant read must succeed first, because a cross-tenant 404 is
    # byte-identical to a missing-row 404 and would pass for the wrong reason.
    assert s.get_resource("org_acme", "r_acme_pg").id == "r_acme_pg"
    with pytest.raises(NotFoundError):
        s.get_resource("org_acme", "r_globex_k8s")


def test_resources_order_by_created_at_then_id():
    s = seeded()
    # The three seeded acme resources share a created_at AND sit in the seed
    # file in id-ascending order, so a stable sort with no tiebreak reproduces
    # the expected answer. These two probes share a timestamp in REVERSE id
    # order, so only a real tiebreak orders them correctly.
    s.register_resource("org_acme", "zz-probe", "", "internal", "u_ada", _id="r_acme_zz", _at=PROBE_AT)
    s.register_resource("org_acme", "aa-probe", "", "internal", "u_ada", _id="r_acme_aa", _at=PROBE_AT)
    ids = [r.id for r in s.list_org_resources("org_acme")]
    assert ids == ["r_acme_pg", "r_acme_s3", "r_acme_stripe", "r_acme_aa", "r_acme_zz"]


def noop(s):
    pass


def approve_first(s):
    # revoke requires status approved (openapi.yaml:556), so the owning-org
    # control needs an approved request to act on. Always uses org_acme, so
    # it never interferes with the scoping under test.
    s.approve_request("org_acme", "req_acme_1", "u_ada")


@pytest.mark.parametrize("name,setup,call", [
    ("get_resource", noop, lambda s, org: s.get_resource(org, "r_acme_pg")),
    ("list_resource_requests", noop, lambda s, org: s.list_resource_requests(org, "r_acme_pg")),
    ("decommission_resource", noop, lambda s, org: s.decommission_resource(org, "r_acme_pg")),
    ("request_access", noop, lambda s, org: s.request_access(org, "r_acme_pg", "u_linus", "why")),
    ("get_request", noop, lambda s, org: s.get_request(org, "req_acme_1")),
    ("approve_request", noop, lambda s, org: s.approve_request(org, "req_acme_1", "u_ada")),
    ("deny_request", noop, lambda s, org: s.deny_request(org, "req_acme_1", "u_ada")),
    ("revoke_request", approve_first, lambda s, org: s.revoke_request(org, "req_acme_1", "u_ada")),
    ("list_request_comments", noop, lambda s, org: s.list_request_comments(org, "req_acme_1")),
    ("add_comment", noop, lambda s, org: s.add_comment(org, "req_acme_1", "u_linus", "hi")),
    ("remove_member", noop, lambda s, org: s.remove_member(org, "u_linus")),
    ("update_member_role", noop, lambda s, org: s.update_member_role(org, "u_linus", "admin")),
])
def test_every_org_scoped_method_refuses_a_cross_tenant_target(name, setup, call):
    owning = seeded()
    setup(owning)
    # Positive control first: the owning org must succeed, or a method that
    # refuses everything would pass the cross-tenant assertion for free.
    call(owning, "org_acme")

    probe = seeded()
    setup(probe)
    with pytest.raises(NotFoundError):
        call(probe, "org_globex")


def test_members_order_by_created_at_then_id():
    s = seeded()
    # Probes share a timestamp in REVERSE id order, so neither an unsorted list
    # nor a stable sort without the tiebreak reproduces this.
    s.invite_member("org_acme", "zz", "zz@acme.test", "member", _id="u_acme_zz", _at=PROBE_AT)
    s.invite_member("org_acme", "aa", "aa@acme.test", "member", _id="u_acme_aa", _at=PROBE_AT)
    ids = [u.id for u in s.list_org_members("org_acme")]
    assert ids == ["u_ada", "u_linus", "u_acme_aa", "u_acme_zz"]


def test_resource_requests_match_the_contract_worked_example():
    # x-ordering.examples publishes this sequence over the fixture.
    s = seeded()
    ids = [r.id for r in s.list_resource_requests("org_acme", "r_acme_pg")]
    assert ids == ["req_acme_1", "req_acme_4", "req_acme_5"]


def test_resource_requests_order_by_created_at_desc_then_id():
    s = seeded()
    # Later than every seeded row, so a missing sort moves them visibly, and
    # tied with each other in REVERSE id order, so the tiebreak is exercised.
    s.request_access("org_acme", "r_acme_pg", "u_linus", "zz", _id="req_acme_zz", _at=PROBE_AT)
    s.request_access("org_acme", "r_acme_pg", "u_linus", "aa", _id="req_acme_aa", _at=PROBE_AT)
    ids = [r.id for r in s.list_resource_requests("org_acme", "r_acme_pg")]
    assert ids == ["req_acme_aa", "req_acme_zz", "req_acme_1", "req_acme_4", "req_acme_5"]


def test_request_comments_match_the_contract_worked_example():
    # The second sequence x-ordering.examples publishes.
    s = seeded()
    ids = [c.id for c in s.list_request_comments("org_acme", "req_acme_1")]
    assert ids == ["c_acme_1", "c_acme_2", "c_acme_3"]


def test_request_comments_order_by_created_at_then_id():
    s = seeded()
    s.add_comment("org_acme", "req_acme_1", "u_ada", "zz", _id="c_acme_zz", _at=PROBE_AT)
    s.add_comment("org_acme", "req_acme_1", "u_ada", "aa", _id="c_acme_aa", _at=PROBE_AT)
    ids = [c.id for c in s.list_request_comments("org_acme", "req_acme_1")]
    assert ids == ["c_acme_1", "c_acme_2", "c_acme_3", "c_acme_aa", "c_acme_zz"]


def test_created_at_is_compared_as_an_instant_not_a_string():
    s = seeded()
    # Same second, one fractional and one whole. As strings "...00.500Z" sorts
    # before "...00Z" because "." < "Z", which is the opposite of the instants.
    s.register_resource("org_acme", "frac", "", "internal", "u_ada",
                        _id="r_acme_m2", _at="2026-03-20T12:00:00.500Z")
    s.register_resource("org_acme", "whole", "", "internal", "u_ada",
                        _id="r_acme_m1", _at="2026-03-20T12:00:00Z")
    assert [r.id for r in s.list_org_resources("org_acme")][-2:] == ["r_acme_m1", "r_acme_m2"]


def test_runtime_timestamps_are_utc_z_with_at_least_millisecond_precision():
    s = seeded()
    # The store must mint through the helper, so check one of each.
    for stamp in (now_rfc3339(), s.add_comment("org_acme", "req_acme_1", "u_ada", "hi").created_at):
        assert stamp.endswith("Z"), stamp
        assert "+" not in stamp, stamp
        assert "." in stamp, stamp
        assert len(stamp.split(".")[1].rstrip("Z")) >= 3, stamp
        assert parse_rfc3339(stamp).utcoffset().total_seconds() == 0


def test_optional_fields_are_omitted_not_null():
    s = seeded()
    pending = s.get_request("org_acme", "req_acme_1").to_dict()
    assert set(pending) == {
        "id", "resource_id", "organization_id", "requested_by", "reason", "status", "created_at",
    }
    live = s.get_resource("org_acme", "r_acme_pg").to_dict()
    assert set(live) == {
        "id", "name", "description", "sensitivity", "organization_id", "registered_by", "created_at",
    }
    gone = s.decommission_resource("org_acme", "r_acme_pg").to_dict()
    assert gone["decommission_at"].endswith("Z")


def test_revoke_preserves_the_approval_decision_fields():
    s = seeded()
    approved = s.approve_request("org_acme", "req_acme_1", "u_ada")
    # The store is role-agnostic, so a second actor id just keeps the two
    # field pairs distinguishable.
    revoked = s.revoke_request("org_acme", "req_acme_1", "u_linus")
    assert revoked.status == "revoked"
    assert revoked.decided_by == "u_ada"
    assert revoked.decided_at == approved.decided_at
    assert revoked.revoked_by == "u_linus"
    assert revoked.revoked_at is not None


def test_duplicate_email_resolves_under_the_contract_order():
    s = seeded()
    # Minted later, so created_at keeps the seeded user ahead of the duplicate.
    s.invite_member("org_globex", "Impostor", "ada@acme.test", "admin", _id="u_dup_1", _at=PROBE_AT)
    assert s.find_user_by_email("ada@acme.test").id == "u_ada"

    # Tied on created_at and inserted in REVERSE id order, so returning the
    # first stored match answers differently from the contract's order.
    s.invite_member("org_acme", "zz", "dup@acme.test", "member", _id="u_dup_zz", _at=PROBE_AT)
    s.invite_member("org_acme", "aa", "dup@acme.test", "member", _id="u_dup_aa", _at=PROBE_AT)
    assert s.find_user_by_email("dup@acme.test").id == "u_dup_aa"
