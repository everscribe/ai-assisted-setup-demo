//! The persistence boundary. MemoryStore is the only implementation today.
//!
//! Two deliberate differences from the Go interface this mirrors. The trait
//! is #[async_trait] because an async fn in a plain trait is not
//! dyn-compatible and the handlers need Arc<dyn Store>. And no method takes
//! a context: request-scoped state travels through axum's extensions, so
//! the seam is the middleware layer rather than an invented argument.

use crate::models::{AccessRequest, Comment, Organization, Resource, User};

/// NotFound covers both a missing row and one in another organization, so
/// the tenant boundary does not leak existence. Forbidden is a
/// structurally disallowed action. Conflict is a state transition that
/// does not apply.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum StoreError {
    NotFound,
    // Never constructed here: a cross-tenant target is NotFound instead.
    // The handlers still map it, so the arm is live and the variant is not.
    #[allow(dead_code)]
    Forbidden,
    Conflict,
}

impl std::fmt::Display for StoreError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(match self {
            StoreError::NotFound => "not found",
            StoreError::Forbidden => "forbidden",
            StoreError::Conflict => "conflict",
        })
    }
}

impl std::error::Error for StoreError {}

#[async_trait::async_trait]
pub trait Store: Send + Sync {
    async fn seed(&self) -> Result<(), StoreError>;
    async fn get_user(&self, id: &str) -> Result<User, StoreError>;
    async fn find_user_by_email(&self, email: &str) -> Result<User, StoreError>;
    async fn list_org_members(&self, org_id: &str) -> Result<Vec<User>, StoreError>;
    async fn invite_member(&self, org_id: &str, name: &str, email: &str, role: &str) -> Result<User, StoreError>;
    async fn remove_member(&self, org_id: &str, user_id: &str) -> Result<(), StoreError>;
    async fn update_member_role(&self, org_id: &str, user_id: &str, role: &str) -> Result<User, StoreError>;
    async fn list_org_resources(&self, org_id: &str) -> Result<Vec<Resource>, StoreError>;
    // No route reaches this one; the tests do.
    #[allow(dead_code)]
    async fn get_resource(&self, org_id: &str, resource_id: &str) -> Result<Resource, StoreError>;
    async fn register_resource(&self, org_id: &str, name: &str, description: &str, sensitivity: &str, registered_by: &str) -> Result<Resource, StoreError>;
    async fn decommission_resource(&self, org_id: &str, resource_id: &str) -> Result<Resource, StoreError>;
    async fn list_resource_requests(&self, org_id: &str, resource_id: &str) -> Result<Vec<AccessRequest>, StoreError>;
    // No route reaches this one; the tests do.
    #[allow(dead_code)]
    async fn get_request(&self, org_id: &str, request_id: &str) -> Result<AccessRequest, StoreError>;
    async fn request_access(&self, org_id: &str, resource_id: &str, requested_by: &str, reason: &str) -> Result<AccessRequest, StoreError>;
    async fn approve_request(&self, org_id: &str, request_id: &str, approved_by: &str) -> Result<AccessRequest, StoreError>;
    async fn deny_request(&self, org_id: &str, request_id: &str, denied_by: &str) -> Result<AccessRequest, StoreError>;
    async fn revoke_request(&self, org_id: &str, request_id: &str, revoked_by: &str) -> Result<AccessRequest, StoreError>;
    async fn list_request_comments(&self, org_id: &str, request_id: &str) -> Result<Vec<Comment>, StoreError>;
    async fn add_comment(&self, org_id: &str, request_id: &str, author_id: &str, body: &str) -> Result<Comment, StoreError>;
    // No route reaches this one; the tests do.
    #[allow(dead_code)]
    async fn list_organizations(&self) -> Result<Vec<Organization>, StoreError>;
    // No route reaches this one; the tests do.
    #[allow(dead_code)]
    async fn get_organization(&self, id: &str) -> Result<Organization, StoreError>;
}
