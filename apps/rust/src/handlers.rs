//! One function per route, plus the four shared helpers.
//!
//! Every handler runs the contract's checks in the pinned order: 401
//! authentication, then 403 admin role, then 400 body parsing and field
//! validation, then whatever 404 or 409 the store returns. The body
//! arrives as raw `Bytes` and is parsed inside the handler on purpose: an
//! extractor that parsed it would reject a malformed body before the
//! handler ran, putting a 400 ahead of the 401 and 403 the contract wants.

use axum::body::{Body, Bytes};
use axum::extract::{Path, State};
use axum::http::{header, HeaderMap, StatusCode};
use axum::response::Response;
use serde::Serialize;
use serde_json::{json, Value};
use std::sync::Arc;

use everscribe::axum::CurrentEvent;
use everscribe::event::Target;

use crate::models::User;
use crate::store::{Store, StoreError};

pub type AppState = Arc<dyn Store>;

/// Every response that has a body carries exactly this.
const JSON: &str = "application/json; charset=utf-8";

pub fn json_response(status: StatusCode, body: &impl Serialize) -> Response {
    let bytes = serde_json::to_vec(body).expect("response serializes");
    Response::builder()
        .status(status)
        .header(header::CONTENT_TYPE, JSON)
        .body(Body::from(bytes))
        .expect("response builds")
}

/// The only error shape in this app.
pub fn fail(status: StatusCode, message: &str) -> Response {
    json_response(status, &json!({ "error": message }))
}

/// Store errors as the contract spells them. Forbidden is here for
/// completeness; the store returns NotFound for a cross-tenant target so
/// the boundary does not leak existence.
pub fn fail_store(err: StoreError) -> Response {
    match err {
        StoreError::NotFound => fail(StatusCode::NOT_FOUND, "not found"),
        StoreError::Forbidden => fail(StatusCode::FORBIDDEN, "forbidden"),
        StoreError::Conflict => fail(StatusCode::CONFLICT, "conflict"),
    }
}

/// 204: no body, and no Content-Type at all.
fn no_content() -> Response {
    Response::builder()
        .status(StatusCode::NO_CONTENT)
        .body(Body::empty())
        .expect("response builds")
}

fn unauthorized() -> Response {
    fail(StatusCode::UNAUTHORIZED, "unauthorized")
}

/// X-User-ID names a user or the request is a 401. The header is matched
/// exactly; the contract trims body fields, never this.
pub async fn require_actor(headers: &HeaderMap, store: &AppState) -> Result<User, Response> {
    let id = headers
        .get("X-User-ID")
        .and_then(|v| v.to_str().ok())
        .unwrap_or_default();
    if id.is_empty() {
        return Err(unauthorized());
    }
    store.get_user(id).await.map_err(|_| unauthorized())
}

pub fn require_admin(user: &User) -> Result<(), Response> {
    if user.role == "admin" {
        Ok(())
    } else {
        Err(fail(StatusCode::FORBIDDEN, "admin role required"))
    }
}

/// The 400 for a body that is not a JSON object. The text is not
/// normative; only the status is.
fn bad_body() -> Response {
    fail(StatusCode::BAD_REQUEST, "invalid body")
}

/// A missing, empty or non-object body is a 400.
fn parse_body(bytes: &Bytes) -> Result<Value, Response> {
    match serde_json::from_slice::<Value>(bytes) {
        Ok(v @ Value::Object(_)) => Ok(v),
        _ => Err(bad_body()),
    }
}

/// A string field. Absent and null both read as empty, which is what a
/// struct-binding decoder does. Any other type is a 400.
fn str_field<'a>(body: &'a Value, key: &str) -> Result<&'a str, Response> {
    match body.get(key) {
        None | Some(Value::Null) => Ok(""),
        Some(Value::String(s)) => Ok(s),
        Some(_) => Err(bad_body()),
    }
}

// ---------------------------------------------------------------- routes

pub async fn login(
    State(store): State<AppState>,
    ev: CurrentEvent,
    body: Bytes,
) -> Result<Response, Response> {
    let b = parse_body(&body)?;
    // Pointedly not trimmed: the email is matched exactly.
    let email = str_field(&b, "email")?;
    if email.is_empty() {
        return Err(fail(StatusCode::BAD_REQUEST, "email required"));
    }

    // Named here, after the body checks but before the lookup, so a
    // rejected sign-in still records: the layer fills the outcome from the
    // response, so the 401 below lands as user.login with an error result
    // and no tenant. The 400s above return without an action and record
    // nothing, which is the intent - a malformed body reached no account.
    ev.with(|e| e.action = "user.login".into());

    let user = store
        .find_user_by_email(email)
        .await
        .map_err(|_| fail(StatusCode::UNAUTHORIZED, "no user with that email"))?;

    // Only known once the email resolves. A failed sign-in has no tenant
    // to attribute, by construction.
    ev.with(|e| {
        e.target = Target::new("user", &user.id);
        e.tenant_id = user.organization_id.clone();
    });

    Ok(json_response(StatusCode::OK, &user))
}

pub async fn logout(
    State(store): State<AppState>,
    ev: CurrentEvent,
    headers: HeaderMap,
) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;

    ev.with(|e| {
        e.action = "user.logout".into();
        e.target = Target::new("user", &actor.id);
        e.tenant_id = actor.organization_id.clone();
    });

    Ok(no_content())
}

pub async fn me(State(store): State<AppState>, headers: HeaderMap) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;
    Ok(json_response(StatusCode::OK, &actor))
}

pub async fn list_members(State(store): State<AppState>, headers: HeaderMap) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;
    let rows = store
        .list_org_members(&actor.organization_id)
        .await
        .map_err(fail_store)?;
    Ok(json_response(StatusCode::OK, &rows))
}

pub async fn invite_member(
    State(store): State<AppState>,
    ev: CurrentEvent,
    headers: HeaderMap,
    body: Bytes,
) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;
    require_admin(&actor)?;
    let b = parse_body(&body)?;
    let name = str_field(&b, "name")?.trim();
    let email = str_field(&b, "email")?.trim();
    if name.is_empty() || email.is_empty() {
        return Err(fail(StatusCode::BAD_REQUEST, "name and email required"));
    }
    // Not trimmed and not validated: anything but admin defaults to member.
    let role = str_field(&b, "role")?;

    // Named before the store call, so a conflict (409, email already in
    // the org) records as a failed invite rather than vanishing.
    ev.with(|e| {
        e.action = "member.invited".into();
        e.tenant_id = actor.organization_id.clone();
        e.metadata.insert("email".into(), email.into());
        e.metadata.insert("role".into(), role.into());
    });

    let user = store
        .invite_member(&actor.organization_id, name, email, role)
        .await
        .map_err(fail_store)?;

    // The new member's id only exists after the store accepts it.
    ev.with(|e| e.target = Target::new("user", &user.id));

    Ok(json_response(StatusCode::CREATED, &user))
}

pub async fn remove_member(
    State(store): State<AppState>,
    ev: CurrentEvent,
    headers: HeaderMap,
    Path(id): Path<String>,
) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;
    require_admin(&actor)?;

    ev.with(|e| {
        e.action = "member.removed".into();
        e.target = Target::new("user", &id);
        e.tenant_id = actor.organization_id.clone();
    });

    store
        .remove_member(&actor.organization_id, &id)
        .await
        .map_err(fail_store)?;
    Ok(no_content())
}

pub async fn change_member_role(
    State(store): State<AppState>,
    ev: CurrentEvent,
    headers: HeaderMap,
    Path(id): Path<String>,
    body: Bytes,
) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;
    require_admin(&actor)?;
    let b = parse_body(&body)?;
    // Compared exactly, so " admin " is a 400 rather than a promotion.
    let role = str_field(&b, "role")?;
    if role != "admin" && role != "member" {
        return Err(fail(StatusCode::BAD_REQUEST, "role must be admin or member"));
    }

    // A privilege change is the event an auditor reads first, so it is
    // named before the store call and carries the requested role even when
    // the call is refused.
    ev.with(|e| {
        e.action = "member.role.changed".into();
        e.target = Target::new("user", &id);
        e.tenant_id = actor.organization_id.clone();
        e.metadata.insert("role".into(), role.into());
    });

    let user = store
        .update_member_role(&actor.organization_id, &id, role)
        .await
        .map_err(fail_store)?;
    Ok(json_response(StatusCode::OK, &user))
}

pub async fn list_resources(State(store): State<AppState>, headers: HeaderMap) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;
    let rows = store
        .list_org_resources(&actor.organization_id)
        .await
        .map_err(fail_store)?;
    Ok(json_response(StatusCode::OK, &rows))
}

pub async fn register_resource(
    State(store): State<AppState>,
    ev: CurrentEvent,
    headers: HeaderMap,
    body: Bytes,
) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;
    require_admin(&actor)?;
    let b = parse_body(&body)?;
    let name = str_field(&b, "name")?.trim();
    if name.is_empty() {
        return Err(fail(StatusCode::BAD_REQUEST, "name required"));
    }
    let description = str_field(&b, "description")?.trim();
    // Trimmed but not required; empty means internal.
    let sensitivity = str_field(&b, "sensitivity")?.trim();

    ev.with(|e| {
        e.action = "resource.registered".into();
        e.tenant_id = actor.organization_id.clone();
        e.metadata.insert("name".into(), name.into());
        e.metadata.insert("sensitivity".into(), sensitivity.into());
    });

    let resource = store
        .register_resource(&actor.organization_id, name, description, sensitivity, &actor.id)
        .await
        .map_err(fail_store)?;

    ev.with(|e| e.target = Target::new("resource", &resource.id));

    Ok(json_response(StatusCode::CREATED, &resource))
}

pub async fn decommission_resource(
    State(store): State<AppState>,
    ev: CurrentEvent,
    headers: HeaderMap,
    Path(id): Path<String>,
) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;
    require_admin(&actor)?;

    ev.with(|e| {
        e.action = "resource.decommissioned".into();
        e.target = Target::new("resource", &id);
        e.tenant_id = actor.organization_id.clone();
    });

    let resource = store
        .decommission_resource(&actor.organization_id, &id)
        .await
        .map_err(fail_store)?;
    Ok(json_response(StatusCode::OK, &resource))
}

pub async fn list_resource_requests(
    State(store): State<AppState>,
    headers: HeaderMap,
    Path(id): Path<String>,
) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;
    let rows = store
        .list_resource_requests(&actor.organization_id, &id)
        .await
        .map_err(fail_store)?;
    Ok(json_response(StatusCode::OK, &rows))
}

pub async fn request_access(
    State(store): State<AppState>,
    ev: CurrentEvent,
    headers: HeaderMap,
    Path(id): Path<String>,
    body: Bytes,
) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;
    // No admin check here, so the body is read before the store is touched
    // and an unparseable body to an unknown resource is a 400, not a 404.
    let b = parse_body(&body)?;
    let reason = str_field(&b, "reason")?.trim();
    if reason.is_empty() {
        return Err(fail(StatusCode::BAD_REQUEST, "reason required"));
    }

    // Target is the resource until the request exists; a refusal
    // (decommissioned resource, or a cross-tenant miss) still says what
    // was being asked for.
    ev.with(|e| {
        e.action = "access_request.submitted".into();
        e.target = Target::new("resource", &id);
        e.tenant_id = actor.organization_id.clone();
        e.metadata.insert("reason".into(), reason.into());
    });

    let request = store
        .request_access(&actor.organization_id, &id, &actor.id, reason)
        .await
        .map_err(fail_store)?;

    ev.with(|e| e.target = Target::new("access_request", &request.id));

    Ok(json_response(StatusCode::CREATED, &request))
}

pub async fn approve_request(
    State(store): State<AppState>,
    ev: CurrentEvent,
    headers: HeaderMap,
    Path(id): Path<String>,
) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;
    require_admin(&actor)?;

    // Named before the store call: a refused decision (already decided,
    // wrong tenant) is as auditable as an accepted one.
    ev.with(|e| {
        e.action = "access_request.approved".into();
        e.target = Target::new("access_request", &id);
        e.tenant_id = actor.organization_id.clone();
    });

    let request = store
        .approve_request(&actor.organization_id, &id, &actor.id)
        .await
        .map_err(fail_store)?;
    Ok(json_response(StatusCode::OK, &request))
}

pub async fn deny_request(
    State(store): State<AppState>,
    ev: CurrentEvent,
    headers: HeaderMap,
    Path(id): Path<String>,
) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;
    require_admin(&actor)?;

    // Named before the store call: a refused decision (already decided,
    // wrong tenant) is as auditable as an accepted one.
    ev.with(|e| {
        e.action = "access_request.denied".into();
        e.target = Target::new("access_request", &id);
        e.tenant_id = actor.organization_id.clone();
    });

    let request = store
        .deny_request(&actor.organization_id, &id, &actor.id)
        .await
        .map_err(fail_store)?;
    Ok(json_response(StatusCode::OK, &request))
}

pub async fn revoke_request(
    State(store): State<AppState>,
    ev: CurrentEvent,
    headers: HeaderMap,
    Path(id): Path<String>,
) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;
    require_admin(&actor)?;

    // Named before the store call: a refused decision (already decided,
    // wrong tenant) is as auditable as an accepted one.
    ev.with(|e| {
        e.action = "access_request.revoked".into();
        e.target = Target::new("access_request", &id);
        e.tenant_id = actor.organization_id.clone();
    });

    let request = store
        .revoke_request(&actor.organization_id, &id, &actor.id)
        .await
        .map_err(fail_store)?;
    Ok(json_response(StatusCode::OK, &request))
}

pub async fn list_request_comments(
    State(store): State<AppState>,
    headers: HeaderMap,
    Path(id): Path<String>,
) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;
    let rows = store
        .list_request_comments(&actor.organization_id, &id)
        .await
        .map_err(fail_store)?;
    Ok(json_response(StatusCode::OK, &rows))
}

pub async fn add_comment(
    State(store): State<AppState>,
    ev: CurrentEvent,
    headers: HeaderMap,
    Path(id): Path<String>,
    body: Bytes,
) -> Result<Response, Response> {
    let actor = require_actor(&headers, &store).await?;
    let b = parse_body(&body)?;
    let text = str_field(&b, "body")?.trim();
    if text.is_empty() {
        return Err(fail(StatusCode::BAD_REQUEST, "body required"));
    }

    // The comment is part of the approval trail, so the event points at
    // the access request rather than at the comment row. The text is
    // deliberately not copied into metadata: it is free text a requester
    // typed, and the audit log is not the place to duplicate it.
    ev.with(|e| {
        e.action = "access_request.commented".into();
        e.target = Target::new("access_request", &id);
        e.tenant_id = actor.organization_id.clone();
    });

    let comment = store
        .add_comment(&actor.organization_id, &id, &actor.id, text)
        .await
        .map_err(fail_store)?;
    Ok(json_response(StatusCode::CREATED, &comment))
}

pub async fn health() -> Response {
    json_response(StatusCode::OK, &json!({ "status": "ok" }))
}

/// Unmatched path, including a trailing slash or the wrong case. A HEAD
/// gets this too, headers and all: the HTTP layer drops the body, which is
/// the only thing RFC 9110 4.3.2 forbids.
pub async fn not_found() -> Response {
    fail(StatusCode::NOT_FOUND, "not found")
}
