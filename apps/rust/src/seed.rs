//! The fixture from contract/seed.json, hardcoded. Each app owns its seed
//! and the harness asserts they agree, so nothing is read at runtime.

use chrono::{DateTime, Utc};

use crate::models::{AccessRequest, Comment, Organization, Resource, User};

fn ts(s: &str) -> DateTime<Utc> {
    DateTime::parse_from_rfc3339(s)
        .expect("seed timestamp is RFC 3339")
        .with_timezone(&Utc)
}

fn org(id: &str, name: &str, created_at: &str) -> Organization {
    Organization { id: id.into(), name: name.into(), created_at: ts(created_at) }
}

fn user(id: &str, name: &str, email: &str, role: &str, org_id: &str, created_at: &str) -> User {
    User {
        id: id.into(),
        name: name.into(),
        email: email.into(),
        role: role.into(),
        organization_id: org_id.into(),
        created_at: ts(created_at),
    }
}

fn resource(
    id: &str,
    name: &str,
    description: &str,
    sensitivity: &str,
    org_id: &str,
    registered_by: &str,
    created_at: &str,
) -> Resource {
    Resource {
        id: id.into(),
        name: name.into(),
        description: description.into(),
        sensitivity: sensitivity.into(),
        organization_id: org_id.into(),
        registered_by: registered_by.into(),
        created_at: ts(created_at),
        decommission_at: None,
    }
}

fn request(
    id: &str,
    resource_id: &str,
    org_id: &str,
    requested_by: &str,
    reason: &str,
    status: &str,
    decided: Option<(&str, &str)>,
    revoked: Option<(&str, &str)>,
    created_at: &str,
) -> AccessRequest {
    AccessRequest {
        id: id.into(),
        resource_id: resource_id.into(),
        organization_id: org_id.into(),
        requested_by: requested_by.into(),
        reason: reason.into(),
        status: status.into(),
        decided_by: decided.map(|(by, _)| by.into()),
        decided_at: decided.map(|(_, at)| ts(at)),
        revoked_by: revoked.map(|(by, _)| by.into()),
        revoked_at: revoked.map(|(_, at)| ts(at)),
        created_at: ts(created_at),
    }
}

fn comment(id: &str, request_id: &str, org_id: &str, author_id: &str, body: &str, created_at: &str) -> Comment {
    Comment {
        id: id.into(),
        request_id: request_id.into(),
        organization_id: org_id.into(),
        author_id: author_id.into(),
        body: body.into(),
        created_at: ts(created_at),
    }
}

pub fn organizations() -> Vec<Organization> {
    vec![
        org("org_acme", "Acme Corp", "2026-03-13T12:00:00Z"),
        org("org_globex", "Globex", "2026-03-13T12:00:00Z"),
    ]
}

pub fn users() -> Vec<User> {
    vec![
        user("u_ada", "Ada Lovelace", "ada@acme.test", "admin", "org_acme", "2026-03-13T12:00:00Z"),
        user("u_alan", "Alan Turing", "alan@globex.test", "member", "org_globex", "2026-03-13T12:00:00Z"),
        user("u_grace", "Grace Hopper", "grace@globex.test", "admin", "org_globex", "2026-03-13T12:00:00Z"),
        user("u_linus", "Linus Torvalds", "linus@acme.test", "member", "org_acme", "2026-03-13T12:00:00Z"),
    ]
}

pub fn resources() -> Vec<Resource> {
    vec![
        resource(
            "r_acme_pg",
            "production-postgres",
            "Primary customer database (us-east-1)",
            "restricted",
            "org_acme",
            "u_ada",
            "2026-03-13T12:00:00Z",
        ),
        resource(
            "r_acme_s3",
            "audit-logs-bucket",
            "S3 bucket for archived audit logs",
            "confidential",
            "org_acme",
            "u_ada",
            "2026-03-13T12:00:00Z",
        ),
        resource(
            "r_acme_stripe",
            "stripe-live-dashboard",
            "Live-mode Stripe console - refund + chargeback ops",
            "restricted",
            "org_acme",
            "u_ada",
            "2026-03-13T12:00:00Z",
        ),
        resource(
            "r_globex_k8s",
            "k8s-prod-cluster",
            "Production EKS cluster (kubectl admin)",
            "restricted",
            "org_globex",
            "u_grace",
            "2026-03-13T12:00:00Z",
        ),
        resource(
            "r_globex_pagerduty",
            "pagerduty-admin",
            "PagerDuty account admin (rotation + escalation policies)",
            "confidential",
            "org_globex",
            "u_grace",
            "2026-03-13T12:00:00Z",
        ),
    ]
}

pub fn access_requests() -> Vec<AccessRequest> {
    vec![
        request(
            "req_acme_1",
            "r_acme_pg",
            "org_acme",
            "u_linus",
            "Debug a stuck payment intent (#PAY-4821).",
            "pending",
            None,
            None,
            "2026-03-14T12:00:00Z",
        ),
        request(
            "req_acme_2",
            "r_acme_stripe",
            "org_acme",
            "u_linus",
            "Process a chargeback for customer ACME-203.",
            "approved",
            Some(("u_ada", "2026-03-14T10:00:00Z")),
            None,
            "2026-03-14T09:00:00Z",
        ),
        request(
            "req_acme_3",
            "r_acme_s3",
            "org_acme",
            "u_linus",
            "Pull logs for the SOC2 evidence locker.",
            "denied",
            Some(("u_ada", "2026-03-13T12:00:00Z")),
            None,
            "2026-03-13T11:00:00Z",
        ),
        request(
            "req_acme_4",
            "r_acme_pg",
            "org_acme",
            "u_linus",
            "Re-run last quarter's revenue backfill.",
            "denied",
            Some(("u_ada", "2026-03-14T12:00:00Z")),
            None,
            "2026-03-14T12:00:00Z",
        ),
        request(
            "req_acme_5",
            "r_acme_pg",
            "org_acme",
            "u_linus",
            "Verify the read replica lag alert.",
            "approved",
            Some(("u_ada", "2026-03-14T12:00:00Z")),
            None,
            "2026-03-14T10:00:00Z",
        ),
        request(
            "req_globex_1",
            "r_globex_k8s",
            "org_globex",
            "u_alan",
            "Roll a hotfix for the on-call escalation bug.",
            "approved",
            Some(("u_grace", "2026-03-14T10:00:00Z")),
            None,
            "2026-03-14T09:30:00Z",
        ),
        request(
            "req_globex_2",
            "r_globex_pagerduty",
            "org_globex",
            "u_alan",
            "Update the after-hours escalation policy.",
            "revoked",
            Some(("u_grace", "2026-03-13T12:00:00Z")),
            Some(("u_grace", "2026-03-14T10:00:00Z")),
            "2026-03-13T10:00:00Z",
        ),
    ]
}

pub fn comments() -> Vec<Comment> {
    vec![
        comment(
            "c_acme_1",
            "req_acme_1",
            "org_acme",
            "u_linus",
            "Scope is the payments schema, read-only.",
            "2026-03-14T10:00:00Z",
        ),
        comment(
            "c_acme_2",
            "req_acme_1",
            "org_acme",
            "u_ada",
            "Confirmed with the on-call lead. Approving for 24 hours.",
            "2026-03-14T12:00:00Z",
        ),
        comment(
            "c_acme_3",
            "req_acme_1",
            "org_acme",
            "u_linus",
            "Will drop the grant as soon as PAY-4821 closes.",
            "2026-03-14T12:00:00Z",
        ),
        comment(
            "c_globex_1",
            "req_globex_2",
            "org_globex",
            "u_grace",
            "Revoked once the rotation change landed.",
            "2026-03-14T10:00:00Z",
        ),
    ]
}
