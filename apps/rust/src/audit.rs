//! Everscribe wiring: the actor layer the audit layer resolves from, and
//! the embed-token route the frontend's audit-trail component POSTs for a
//! short-lived credential.
//!
//! `EverscribeLayer`'s resolver is `Fn(&Parts) -> Actor` - synchronous -
//! but `Store::get_user` is async, so the lookup cannot happen inside it.
//! This app also has no session layer (`require_actor` runs inside each
//! handler), so there was nothing in the request extensions to read either.
//! `attach_actor` is that missing upstream step: it resolves `X-User-ID`
//! once per request and inserts the user into the extensions, and
//! `main.rs` adds it AFTER `EverscribeLayer` so it wraps the outside and
//! runs first. Added the other way round, every event would record as an
//! anonymous actor and nothing in the build or the tests would catch it.
//!
//! Named `audit` rather than `everscribe` so the module cannot collide with
//! the `everscribe` crate this file imports from.

use axum::extract::{Request, State};
use axum::http::{header, request::Parts, HeaderMap, StatusCode};
use axum::middleware::Next;
use axum::response::Response;
use everscribe::event::Actor;
use everscribe::minter::{Client as MinterClient, TokenOptions};
use std::sync::Arc;

use crate::handlers::{fail, AppState};
use crate::models::User;
use crate::store::Store;

/// Resolves `X-User-ID` into the request extensions. Never rejects:
/// authorization stays in each handler's `require_actor`, so the
/// contract's check order is untouched and `POST /api/login` (which
/// legitimately arrives with no `X-User-ID`) still reaches its handler.
pub async fn attach_actor(State(store): State<AppState>, mut req: Request, next: Next) -> Response {
    let id = req
        .headers()
        .get("X-User-ID")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("")
        .to_string();

    if !id.is_empty() {
        if let Ok(user) = store.get_user(&id).await {
            req.extensions_mut().insert(user);
        }
    }

    next.run(req).await
}

/// Derives the audit `Actor` from what `attach_actor` inserted.
///
/// `Actor::r#type` uses the SDK's conventional values: "admin" for this
/// app's admin role, "user" for everyone else, "anonymous" when nothing
/// resolved.
pub fn resolve_actor(parts: &Parts) -> Actor {
    match parts.extensions.get::<User>() {
        None => Actor::new("anonymous"),
        Some(user) => Actor {
            r#type: if user.role == "admin" { "admin" } else { "user" }.into(),
            id: user.id.clone(),
            display_name: user.name.clone(),
            email: user.email.clone(),
        },
    }
}

/// Mints the short-lived credential the embeddable audit-trail component
/// needs, scoped to the caller's organization.
///
/// POST, not GET: the component POSTs with no body, so a GET-only route
/// would answer 405 and the embed would never load. Minting also issues a
/// bearer credential and consumes quota, so it is not a safe idempotent
/// read. `Cache-Control: no-store` on top, as defense in depth - a POST
/// response is not cached without an explicit opt-in, but a bearer token
/// sitting in a shared cache is the whole risk this route carries.
pub async fn embed_token(
    State((store, minter)): State<(AppState, Arc<MinterClient>)>,
    headers: HeaderMap,
) -> Response {
    let mut response = mint(&store, &minter, &headers).await;
    response.headers_mut().insert(
        header::CACHE_CONTROL,
        "no-store".parse().expect("static header value"),
    );
    response
}

async fn mint(store: &AppState, minter: &MinterClient, headers: &HeaderMap) -> Response {
    let id = match headers.get("X-User-ID").and_then(|v| v.to_str().ok()) {
        Some(id) if !id.is_empty() => id,
        _ => return fail(StatusCode::UNAUTHORIZED, "unauthorized"),
    };
    let user = match store.get_user(id).await {
        Ok(user) => user,
        Err(_) => return fail(StatusCode::UNAUTHORIZED, "unauthorized"),
    };

    // Scoped to the caller's own organization: the token must not let the
    // component read another tenant's trail.
    let opts = TokenOptions {
        tenant_id: user.organization_id.clone(),
        ..Default::default()
    };
    match minter.mint_token(&opts).await {
        Ok(token) => {
            crate::handlers::json_response(StatusCode::OK, &serde_json::json!({ "token": token }))
        }
        Err(_) => fail(StatusCode::BAD_GATEWAY, "could not mint embed token"),
    }
}
