"""The demo fixture, hardcoded from contract/seed.json.

Each app owns its own copy and the harness asserts the four agree, so this is
never read from disk at runtime. Every timestamp here is whole-second RFC 3339
with a Z suffix, exactly as the fixture writes it.
"""

from app.models import AccessRequest, Comment, Organization, Resource, User

SEEDED_AT = "2026-03-13T12:00:00Z"


def organizations() -> list[Organization]:
    return [
        Organization(id="org_acme", name="Acme Corp", created_at=SEEDED_AT),
        Organization(id="org_globex", name="Globex", created_at=SEEDED_AT),
    ]


def users() -> list[User]:
    return [
        User(id="u_ada", name="Ada Lovelace", email="ada@acme.test", role="admin",
             organization_id="org_acme", created_at=SEEDED_AT),
        User(id="u_alan", name="Alan Turing", email="alan@globex.test", role="member",
             organization_id="org_globex", created_at=SEEDED_AT),
        User(id="u_grace", name="Grace Hopper", email="grace@globex.test", role="admin",
             organization_id="org_globex", created_at=SEEDED_AT),
        User(id="u_linus", name="Linus Torvalds", email="linus@acme.test", role="member",
             organization_id="org_acme", created_at=SEEDED_AT),
    ]


def resources() -> list[Resource]:
    return [
        Resource(id="r_acme_pg", name="production-postgres",
                 description="Primary customer database (us-east-1)",
                 sensitivity="restricted", organization_id="org_acme",
                 registered_by="u_ada", created_at=SEEDED_AT),
        Resource(id="r_acme_s3", name="audit-logs-bucket",
                 description="S3 bucket for archived audit logs",
                 sensitivity="confidential", organization_id="org_acme",
                 registered_by="u_ada", created_at=SEEDED_AT),
        Resource(id="r_acme_stripe", name="stripe-live-dashboard",
                 description="Live-mode Stripe console - refund + chargeback ops",
                 sensitivity="restricted", organization_id="org_acme",
                 registered_by="u_ada", created_at=SEEDED_AT),
        Resource(id="r_globex_k8s", name="k8s-prod-cluster",
                 description="Production EKS cluster (kubectl admin)",
                 sensitivity="restricted", organization_id="org_globex",
                 registered_by="u_grace", created_at=SEEDED_AT),
        Resource(id="r_globex_pagerduty", name="pagerduty-admin",
                 description="PagerDuty account admin (rotation + escalation policies)",
                 sensitivity="confidential", organization_id="org_globex",
                 registered_by="u_grace", created_at=SEEDED_AT),
    ]


def access_requests() -> list[AccessRequest]:
    return [
        AccessRequest(id="req_acme_1", resource_id="r_acme_pg", organization_id="org_acme",
                      requested_by="u_linus",
                      reason="Debug a stuck payment intent (#PAY-4821).",
                      status="pending", created_at="2026-03-14T12:00:00Z"),
        AccessRequest(id="req_acme_2", resource_id="r_acme_stripe", organization_id="org_acme",
                      requested_by="u_linus",
                      reason="Process a chargeback for customer ACME-203.",
                      status="approved", decided_by="u_ada",
                      decided_at="2026-03-14T10:00:00Z",
                      created_at="2026-03-14T09:00:00Z"),
        AccessRequest(id="req_acme_3", resource_id="r_acme_s3", organization_id="org_acme",
                      requested_by="u_linus",
                      reason="Pull logs for the SOC2 evidence locker.",
                      status="denied", decided_by="u_ada",
                      decided_at="2026-03-13T12:00:00Z",
                      created_at="2026-03-13T11:00:00Z"),
        AccessRequest(id="req_acme_4", resource_id="r_acme_pg", organization_id="org_acme",
                      requested_by="u_linus",
                      reason="Re-run last quarter's revenue backfill.",
                      status="denied", decided_by="u_ada",
                      decided_at="2026-03-14T12:00:00Z",
                      created_at="2026-03-14T12:00:00Z"),
        AccessRequest(id="req_acme_5", resource_id="r_acme_pg", organization_id="org_acme",
                      requested_by="u_linus",
                      reason="Verify the read replica lag alert.",
                      status="approved", decided_by="u_ada",
                      decided_at="2026-03-14T12:00:00Z",
                      created_at="2026-03-14T10:00:00Z"),
        AccessRequest(id="req_globex_1", resource_id="r_globex_k8s", organization_id="org_globex",
                      requested_by="u_alan",
                      reason="Roll a hotfix for the on-call escalation bug.",
                      status="approved", decided_by="u_grace",
                      decided_at="2026-03-14T10:00:00Z",
                      created_at="2026-03-14T09:30:00Z"),
        AccessRequest(id="req_globex_2", resource_id="r_globex_pagerduty", organization_id="org_globex",
                      requested_by="u_alan",
                      reason="Update the after-hours escalation policy.",
                      status="revoked", decided_by="u_grace",
                      decided_at="2026-03-13T12:00:00Z",
                      revoked_by="u_grace", revoked_at="2026-03-14T10:00:00Z",
                      created_at="2026-03-13T10:00:00Z"),
    ]


def comments() -> list[Comment]:
    return [
        Comment(id="c_acme_1", request_id="req_acme_1", organization_id="org_acme",
                author_id="u_linus", body="Scope is the payments schema, read-only.",
                created_at="2026-03-14T10:00:00Z"),
        Comment(id="c_acme_2", request_id="req_acme_1", organization_id="org_acme",
                author_id="u_ada",
                body="Confirmed with the on-call lead. Approving for 24 hours.",
                created_at="2026-03-14T12:00:00Z"),
        Comment(id="c_acme_3", request_id="req_acme_1", organization_id="org_acme",
                author_id="u_linus",
                body="Will drop the grant as soon as PAY-4821 closes.",
                created_at="2026-03-14T12:00:00Z"),
        Comment(id="c_globex_1", request_id="req_globex_2", organization_id="org_globex",
                author_id="u_grace", body="Revoked once the rotation change landed.",
                created_at="2026-03-14T10:00:00Z"),
    ]
