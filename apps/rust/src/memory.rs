//! In-memory Store. Every collection is a Vec behind one RwLock.
//!
//! Three rules taken from the contract. Ids are a type prefix plus random
//! hex. Every list is a total order: the domain key, then id ascending,
//! with created_at compared as a parsed instant rather than as a string.
//! Tenant scoping lives here, and a cross-tenant target returns NotFound,
//! identical to a missing row, so the boundary does not leak existence.
//!
//! Vec rather than HashMap because iteration order has to be defined. A
//! HashMap would hand the sort an arbitrary order, so dropping the id
//! tiebreak would produce a different answer run to run instead of a
//! reproducibly wrong one, and the ordering test could not pin it.

use chrono::Utc;
use tokio::sync::RwLock;

use crate::models::{AccessRequest, Comment, Organization, Resource, User};
use crate::seed;
use crate::store::{Store, StoreError};

#[derive(Default)]
struct Data {
    organizations: Vec<Organization>,
    users: Vec<User>,
    resources: Vec<Resource>,
    requests: Vec<AccessRequest>,
    comments: Vec<Comment>,
}

pub struct MemoryStore {
    data: RwLock<Data>,
}

impl MemoryStore {
    pub fn new() -> Self {
        Self { data: RwLock::new(Data::default()) }
    }
}

impl Default for MemoryStore {
    fn default() -> Self {
        Self::new()
    }
}

/// Type prefix plus random hex.
fn new_id(prefix: &str) -> String {
    format!("{}{:016x}", prefix, rand::random::<u64>())
}

/// A resource the org owns, or NotFound.
fn resource_in_org<'a>(d: &'a Data, org_id: &str, resource_id: &str) -> Result<&'a Resource, StoreError> {
    d.resources
        .iter()
        .find(|r| r.id == resource_id && r.organization_id == org_id)
        .ok_or(StoreError::NotFound)
}

/// A request the org owns, or NotFound.
fn request_in_org<'a>(d: &'a Data, org_id: &str, request_id: &str) -> Result<&'a AccessRequest, StoreError> {
    d.requests
        .iter()
        .find(|r| r.id == request_id && r.organization_id == org_id)
        .ok_or(StoreError::NotFound)
}

fn request_in_org_mut<'a>(
    d: &'a mut Data,
    org_id: &str,
    request_id: &str,
) -> Result<&'a mut AccessRequest, StoreError> {
    d.requests
        .iter_mut()
        .find(|r| r.id == request_id && r.organization_id == org_id)
        .ok_or(StoreError::NotFound)
}

/// Decide a pending request. Shared by approve and deny.
fn decide(d: &mut Data, org_id: &str, request_id: &str, by: &str, status: &str) -> Result<AccessRequest, StoreError> {
    let r = request_in_org_mut(d, org_id, request_id)?;
    if r.status != "pending" {
        return Err(StoreError::Conflict);
    }
    r.status = status.into();
    r.decided_by = Some(by.into());
    r.decided_at = Some(Utc::now());
    Ok(r.clone())
}

#[async_trait::async_trait]
impl Store for MemoryStore {
    async fn seed(&self) -> Result<(), StoreError> {
        let mut d = self.data.write().await;
        *d = Data {
            organizations: seed::organizations(),
            users: seed::users(),
            resources: seed::resources(),
            requests: seed::access_requests(),
            comments: seed::comments(),
        };
        Ok(())
    }

    async fn get_user(&self, id: &str) -> Result<User, StoreError> {
        let d = self.data.read().await;
        d.users.iter().find(|u| u.id == id).cloned().ok_or(StoreError::NotFound)
    }

    async fn find_user_by_email(&self, email: &str) -> Result<User, StoreError> {
        let d = self.data.read().await;
        // Emails are not unique, so pick by the same total order the lists use.
        let mut hits: Vec<&User> = d.users.iter().filter(|u| u.email == email).collect();
        hits.sort_by(|a, b| a.created_at.cmp(&b.created_at).then_with(|| a.id.cmp(&b.id)));
        hits.first().map(|u| (*u).clone()).ok_or(StoreError::NotFound)
    }

    async fn list_org_members(&self, org_id: &str) -> Result<Vec<User>, StoreError> {
        let d = self.data.read().await;
        let mut rows: Vec<User> = d.users.iter().filter(|u| u.organization_id == org_id).cloned().collect();
        rows.sort_by(|a, b| a.created_at.cmp(&b.created_at).then_with(|| a.id.cmp(&b.id)));
        Ok(rows)
    }

    async fn invite_member(&self, org_id: &str, name: &str, email: &str, role: &str) -> Result<User, StoreError> {
        let mut d = self.data.write().await;
        // Anything but admin is a member. Email uniqueness is not enforced.
        let role = if role == "admin" { "admin" } else { "member" };
        let u = User {
            id: new_id("u_"),
            name: name.into(),
            email: email.into(),
            role: role.into(),
            organization_id: org_id.into(),
            created_at: Utc::now(),
        };
        d.users.push(u.clone());
        Ok(u)
    }

    async fn remove_member(&self, org_id: &str, user_id: &str) -> Result<(), StoreError> {
        let mut d = self.data.write().await;
        let i = d
            .users
            .iter()
            .position(|u| u.id == user_id && u.organization_id == org_id)
            .ok_or(StoreError::NotFound)?;
        // Their requests and comments stay; the contract never asks for a cascade.
        d.users.remove(i);
        Ok(())
    }

    async fn update_member_role(&self, org_id: &str, user_id: &str, role: &str) -> Result<User, StoreError> {
        let mut d = self.data.write().await;
        let u = d
            .users
            .iter_mut()
            .find(|u| u.id == user_id && u.organization_id == org_id)
            .ok_or(StoreError::NotFound)?;
        u.role = role.into();
        Ok(u.clone())
    }

    async fn list_org_resources(&self, org_id: &str) -> Result<Vec<Resource>, StoreError> {
        let d = self.data.read().await;
        // Decommissioned resources stay in the list.
        let mut rows: Vec<Resource> =
            d.resources.iter().filter(|r| r.organization_id == org_id).cloned().collect();
        rows.sort_by(|a, b| a.created_at.cmp(&b.created_at).then_with(|| a.id.cmp(&b.id)));
        Ok(rows)
    }

    async fn get_resource(&self, org_id: &str, resource_id: &str) -> Result<Resource, StoreError> {
        let d = self.data.read().await;
        resource_in_org(&d, org_id, resource_id).cloned()
    }

    async fn register_resource(
        &self,
        org_id: &str,
        name: &str,
        description: &str,
        sensitivity: &str,
        registered_by: &str,
    ) -> Result<Resource, StoreError> {
        let mut d = self.data.write().await;
        let sensitivity = if sensitivity.is_empty() { "internal" } else { sensitivity };
        let r = Resource {
            id: new_id("r_"),
            name: name.into(),
            description: description.into(),
            sensitivity: sensitivity.into(),
            organization_id: org_id.into(),
            registered_by: registered_by.into(),
            created_at: Utc::now(),
            decommission_at: None,
        };
        d.resources.push(r.clone());
        Ok(r)
    }

    async fn decommission_resource(&self, org_id: &str, resource_id: &str) -> Result<Resource, StoreError> {
        let mut d = self.data.write().await;
        let r = d
            .resources
            .iter_mut()
            .find(|r| r.id == resource_id && r.organization_id == org_id)
            .ok_or(StoreError::NotFound)?;
        // Idempotent: a second call re-stamps rather than conflicting.
        r.decommission_at = Some(Utc::now());
        Ok(r.clone())
    }

    async fn list_resource_requests(&self, org_id: &str, resource_id: &str) -> Result<Vec<AccessRequest>, StoreError> {
        let d = self.data.read().await;
        resource_in_org(&d, org_id, resource_id)?;
        let mut rows: Vec<AccessRequest> = d
            .requests
            .iter()
            .filter(|r| r.organization_id == org_id && r.resource_id == resource_id)
            .cloned()
            .collect();
        rows.sort_by(|a, b| b.created_at.cmp(&a.created_at).then_with(|| a.id.cmp(&b.id)));
        Ok(rows)
    }

    async fn get_request(&self, org_id: &str, request_id: &str) -> Result<AccessRequest, StoreError> {
        let d = self.data.read().await;
        request_in_org(&d, org_id, request_id).cloned()
    }

    async fn request_access(
        &self,
        org_id: &str,
        resource_id: &str,
        requested_by: &str,
        reason: &str,
    ) -> Result<AccessRequest, StoreError> {
        let mut d = self.data.write().await;
        let res = resource_in_org(&d, org_id, resource_id)?;
        if res.decommission_at.is_some() {
            return Err(StoreError::Conflict);
        }
        let r = AccessRequest {
            id: new_id("req_"),
            resource_id: resource_id.into(),
            organization_id: org_id.into(),
            requested_by: requested_by.into(),
            reason: reason.into(),
            status: "pending".into(),
            decided_by: None,
            decided_at: None,
            revoked_by: None,
            revoked_at: None,
            created_at: Utc::now(),
        };
        d.requests.push(r.clone());
        Ok(r)
    }

    async fn approve_request(&self, org_id: &str, request_id: &str, approved_by: &str) -> Result<AccessRequest, StoreError> {
        let mut d = self.data.write().await;
        decide(&mut d, org_id, request_id, approved_by, "approved")
    }

    async fn deny_request(&self, org_id: &str, request_id: &str, denied_by: &str) -> Result<AccessRequest, StoreError> {
        let mut d = self.data.write().await;
        decide(&mut d, org_id, request_id, denied_by, "denied")
    }

    async fn revoke_request(&self, org_id: &str, request_id: &str, revoked_by: &str) -> Result<AccessRequest, StoreError> {
        let mut d = self.data.write().await;
        let r = request_in_org_mut(&mut d, org_id, request_id)?;
        if r.status != "approved" {
            return Err(StoreError::Conflict);
        }
        // decided_by and decided_at keep the approval that is being undone.
        r.status = "revoked".into();
        r.revoked_by = Some(revoked_by.into());
        r.revoked_at = Some(Utc::now());
        Ok(r.clone())
    }

    async fn list_request_comments(&self, org_id: &str, request_id: &str) -> Result<Vec<Comment>, StoreError> {
        let d = self.data.read().await;
        request_in_org(&d, org_id, request_id)?;
        let mut rows: Vec<Comment> = d
            .comments
            .iter()
            .filter(|c| c.organization_id == org_id && c.request_id == request_id)
            .cloned()
            .collect();
        rows.sort_by(|a, b| a.created_at.cmp(&b.created_at).then_with(|| a.id.cmp(&b.id)));
        Ok(rows)
    }

    async fn add_comment(&self, org_id: &str, request_id: &str, author_id: &str, body: &str) -> Result<Comment, StoreError> {
        let mut d = self.data.write().await;
        request_in_org(&d, org_id, request_id)?;
        let c = Comment {
            id: new_id("c_"),
            request_id: request_id.into(),
            organization_id: org_id.into(),
            author_id: author_id.into(),
            body: body.into(),
            created_at: Utc::now(),
        };
        d.comments.push(c.clone());
        Ok(c)
    }

    async fn list_organizations(&self) -> Result<Vec<Organization>, StoreError> {
        let d = self.data.read().await;
        let mut rows: Vec<Organization> = d.organizations.clone();
        rows.sort_by(|a, b| a.created_at.cmp(&b.created_at).then_with(|| a.id.cmp(&b.id)));
        Ok(rows)
    }

    async fn get_organization(&self, id: &str) -> Result<Organization, StoreError> {
        let d = self.data.read().await;
        d.organizations.iter().find(|o| o.id == id).cloned().ok_or(StoreError::NotFound)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn seeds_the_fixture_the_contract_declares() {
        let s = MemoryStore::new();
        s.seed().await.unwrap();
        let u = s.get_user("u_ada").await.unwrap();
        assert_eq!(u.email, "ada@acme.test");
        assert_eq!(u.role, "admin");
    }

    #[tokio::test]
    async fn cross_tenant_resource_is_not_found() {
        let s = MemoryStore::new();
        s.seed().await.unwrap();
        // r_globex_k8s belongs to org_globex, not org_acme.
        let err = s.get_resource("org_acme", "r_globex_k8s").await.unwrap_err();
        assert!(matches!(err, StoreError::NotFound));
    }

    #[tokio::test]
    async fn resources_are_ordered_by_created_at_then_id() {
        let s = MemoryStore::new();
        s.seed().await.unwrap();

        // The three seeded acme resources share a created_at and the seed
        // lists them id-ascending, so on their own a sort with no tiebreak
        // at all still passes. These two rows share a created_at in reverse
        // id order, which only the tiebreak can put right.
        {
            let mut d = s.data.write().await;
            for id in ["r_acme_zz", "r_acme_aa"] {
                d.resources.push(Resource {
                    id: id.into(),
                    name: "tiebreak-probe".into(),
                    description: String::new(),
                    sensitivity: "internal".into(),
                    organization_id: "org_acme".into(),
                    registered_by: "u_ada".into(),
                    created_at: chrono::DateTime::parse_from_rfc3339("2026-03-20T12:00:00Z")
                        .unwrap()
                        .with_timezone(&Utc),
                    decommission_at: None,
                });
            }
        }

        let rows = s.list_org_resources("org_acme").await.unwrap();
        let ids: Vec<_> = rows.iter().map(|r| r.id.as_str()).collect();
        assert_eq!(
            ids,
            vec!["r_acme_pg", "r_acme_s3", "r_acme_stripe", "r_acme_aa", "r_acme_zz"]
        );
    }

    /// The instant the probe rows share. Later than every seeded row, so a
    /// descending order must put them first and an ascending one last.
    fn probe_instant() -> chrono::DateTime<Utc> {
        chrono::DateTime::parse_from_rfc3339("2026-03-20T12:00:00Z")
            .unwrap()
            .with_timezone(&Utc)
    }

    #[tokio::test]
    async fn requests_and_comments_match_the_worked_examples() {
        let s = MemoryStore::new();
        s.seed().await.unwrap();

        // The seeded rows alone prove nothing: their tied pairs already sit
        // in id-ascending file order, so no sort at all reproduces both
        // worked examples. Each probe pair shares an instant in reverse id
        // order, and sits on the far side of the seeded rows from where
        // insertion order leaves it.
        {
            let mut d = s.data.write().await;
            for id in ["req_acme_zz", "req_acme_aa"] {
                d.requests.push(AccessRequest {
                    id: id.into(),
                    resource_id: "r_acme_pg".into(),
                    organization_id: "org_acme".into(),
                    requested_by: "u_linus".into(),
                    reason: "tiebreak-probe".into(),
                    status: "pending".into(),
                    decided_by: None,
                    decided_at: None,
                    revoked_by: None,
                    revoked_at: None,
                    created_at: probe_instant(),
                });
            }
            for id in ["c_acme_zz", "c_acme_aa"] {
                d.comments.push(Comment {
                    id: id.into(),
                    request_id: "req_acme_1".into(),
                    organization_id: "org_acme".into(),
                    author_id: "u_linus".into(),
                    body: "tiebreak-probe".into(),
                    created_at: probe_instant(),
                });
            }
        }

        // created_at descending, then id ascending.
        let rows = s.list_resource_requests("org_acme", "r_acme_pg").await.unwrap();
        let ids: Vec<_> = rows.iter().map(|r| r.id.as_str()).collect();
        assert_eq!(
            ids,
            vec!["req_acme_aa", "req_acme_zz", "req_acme_1", "req_acme_4", "req_acme_5"]
        );

        // created_at ascending, then id ascending.
        let rows = s.list_request_comments("org_acme", "req_acme_1").await.unwrap();
        let ids: Vec<_> = rows.iter().map(|c| c.id.as_str()).collect();
        assert_eq!(
            ids,
            vec!["c_acme_1", "c_acme_2", "c_acme_3", "c_acme_aa", "c_acme_zz"]
        );
    }

    /// Every method that takes an org_id and a target id, with the owning org
    /// first as a positive control so a method that refuses everything fails
    /// too. Each row gets a fresh store because several of them mutate.
    #[tokio::test]
    async fn every_org_scoped_method_refuses_a_cross_tenant_target() {
        let mut problems: Vec<String> = Vec::new();

        macro_rules! check {
            ($name:literal, |$s:ident, $org:ident| $call:expr) => {{
                let $s = MemoryStore::new();
                $s.seed().await.unwrap();
                let $org = "org_acme";
                if let Err(e) = $call.await.map(|_| ()) {
                    problems.push(format!("{}: owning org refused with {:?}", $name, e));
                }

                let $s = MemoryStore::new();
                $s.seed().await.unwrap();
                let $org = "org_globex";
                match $call.await.map(|_| ()) {
                    Err(StoreError::NotFound) => {}
                    other => problems.push(format!("{}: cross-tenant gave {:?}", $name, other)),
                }
            }};
        }

        check!("remove_member", |s, org| s.remove_member(org, "u_linus"));
        check!("update_member_role", |s, org| s.update_member_role(org, "u_linus", "admin"));
        check!("get_resource", |s, org| s.get_resource(org, "r_acme_pg"));
        check!("decommission_resource", |s, org| s.decommission_resource(org, "r_acme_pg"));
        check!("list_resource_requests", |s, org| s.list_resource_requests(org, "r_acme_pg"));
        check!("request_access", |s, org| s.request_access(org, "r_acme_pg", "u_linus", "why"));
        check!("get_request", |s, org| s.get_request(org, "req_acme_1"));
        check!("approve_request", |s, org| s.approve_request(org, "req_acme_1", "u_ada"));
        check!("deny_request", |s, org| s.deny_request(org, "req_acme_1", "u_ada"));
        check!("revoke_request", |s, org| s.revoke_request(org, "req_acme_2", "u_ada"));
        check!("list_request_comments", |s, org| s.list_request_comments(org, "req_acme_1"));
        check!("add_comment", |s, org| s.add_comment(org, "req_acme_1", "u_linus", "hi"));

        assert!(problems.is_empty(), "tenant scoping: {}", problems.join("; "));
    }

    #[tokio::test]
    async fn seed_rows_serialize_the_way_the_contract_spells_them() {
        let s = MemoryStore::new();
        s.seed().await.unwrap();

        // Whole-second Z, no fraction, and no decommission_at while live.
        let r = s.get_resource("org_acme", "r_acme_pg").await.unwrap();
        let j = serde_json::to_value(&r).unwrap();
        assert_eq!(j["created_at"], "2026-03-13T12:00:00Z");
        assert!(j.get("decommission_at").is_none());

        // A pending request omits every decision field rather than nulling it.
        let q = s.get_request("org_acme", "req_acme_1").await.unwrap();
        let j = serde_json::to_value(&q).unwrap();
        assert_eq!(j["created_at"], "2026-03-14T12:00:00Z");
        for f in ["decided_by", "decided_at", "revoked_by", "revoked_at"] {
            assert!(j.get(f).is_none(), "{f} should be absent");
        }
    }

    #[tokio::test]
    async fn sub_second_timestamps_keep_their_digits() {
        let s = MemoryStore::new();
        s.seed().await.unwrap();
        let mut r = s.get_resource("org_acme", "r_acme_pg").await.unwrap();
        r.created_at = chrono::DateTime::parse_from_rfc3339("2026-03-13T12:00:00.250Z")
            .unwrap()
            .with_timezone(&Utc);
        let j = serde_json::to_value(&r).unwrap();
        assert_eq!(j["created_at"], "2026-03-13T12:00:00.250Z");
    }
}
