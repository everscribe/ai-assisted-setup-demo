import pytest

from app.memory import MemoryStore
from app.store import NotFoundError

PROBE_AT = "2026-03-20T12:00:00Z"


def seeded():
    s = MemoryStore()
    s.seed()
    return s


def test_seeds_the_fixture_the_contract_declares():
    s = seeded()
    u = s.get_user("u_ada")
    assert u.email == "ada@acme.test"
    assert u.role == "admin"


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
