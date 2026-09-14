//! The eighteen routes the contract declares, and the two methods it
//! registers on none of them.
//!
//! Two things axum does not do by default. A HEAD falls through to the GET
//! handler unless the HEAD slot is filled, and the contract wants a 405, so
//! every route registers HEAD explicitly. And a method mismatch gets no
//! Allow header, so every route carries its own 405 fallback listing the
//! methods the contract registers for that path. The header's order is
//! unspecified, so these read in the order the contract lists them.

use axum::http::{header, Method, StatusCode};
use axum::response::Response;
use axum::routing::{delete, on, post, put, MethodFilter, MethodRouter};
use axum::Router;

use crate::handlers::{self as h, AppState};
use crate::store::Store;
use std::sync::Arc;

/// 405 with the path's registered methods. A HEAD keeps the status and the
/// Allow header and drops the body, which RFC 9110 4.3.2 requires.
fn method_not_allowed(method: &Method, allow: &'static str) -> Response {
    if method == Method::HEAD {
        return Response::builder()
            .status(StatusCode::METHOD_NOT_ALLOWED)
            .header(header::ALLOW, allow)
            .body(axum::body::Body::empty())
            .expect("response builds");
    }
    let mut res = h::fail(StatusCode::METHOD_NOT_ALLOWED, "method not allowed");
    res.headers_mut()
        .insert(header::ALLOW, allow.parse().expect("static header value"));
    res
}

/// Close a path to every method but the ones registered on it.
fn closed(methods: MethodRouter<AppState>, allow: &'static str) -> MethodRouter<AppState> {
    methods
        .on(MethodFilter::HEAD, move |m: Method| async move {
            method_not_allowed(&m, allow)
        })
        .fallback(move |m: Method| async move { method_not_allowed(&m, allow) })
}

pub fn build_router(store: Arc<dyn Store>) -> Router {
    // Nested as a service so that /api and /api/ reach the API's own JSON
    // 404 rather than a redirect from the mount.
    Router::new()
        .nest_service("/api", api(store))
        .fallback(h::not_found)
}

fn api(store: AppState) -> Router {
    Router::new()
        .route("/login", closed(post(h::login), "POST"))
        .route("/logout", closed(post(h::logout), "POST"))
        .route("/me", closed(on(MethodFilter::GET, h::me), "GET"))
        .route(
            "/members",
            closed(on(MethodFilter::GET, h::list_members).post(h::invite_member), "GET, POST"),
        )
        .route("/members/:id", closed(delete(h::remove_member), "DELETE"))
        .route("/members/:id/role", closed(put(h::change_member_role), "PUT"))
        .route(
            "/resources",
            closed(on(MethodFilter::GET, h::list_resources).post(h::register_resource), "GET, POST"),
        )
        .route(
            "/resources/:id/decommission",
            closed(post(h::decommission_resource), "POST"),
        )
        .route(
            "/resources/:id/requests",
            closed(
                on(MethodFilter::GET, h::list_resource_requests).post(h::request_access),
                "GET, POST",
            ),
        )
        .route("/requests/:id/approve", closed(post(h::approve_request), "POST"))
        .route("/requests/:id/deny", closed(post(h::deny_request), "POST"))
        .route("/requests/:id/revoke", closed(post(h::revoke_request), "POST"))
        .route(
            "/requests/:id/comments",
            closed(
                on(MethodFilter::GET, h::list_request_comments).post(h::add_comment),
                "GET, POST",
            ),
        )
        .route("/health", closed(on(MethodFilter::GET, h::health), "GET"))
        .fallback(h::not_found)
        .with_state(store)
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::body::Body;
    use axum::http::{HeaderMap, Request, StatusCode};
    use serde_json::Value;
    use tower::ServiceExt;

    async fn app() -> axum::Router {
        let store = std::sync::Arc::new(crate::memory::MemoryStore::new());
        store.seed().await.unwrap();
        build_router(store)
    }

    struct Res {
        status: StatusCode,
        headers: HeaderMap,
        body: String,
    }

    impl Res {
        fn json(&self) -> Value {
            serde_json::from_str(&self.body).expect("body is json")
        }
        fn header(&self, name: &str) -> Option<String> {
            self.headers.get(name).map(|v| v.to_str().unwrap().to_string())
        }
        /// Allow compared as a set, because its order is unspecified. Read
        /// through get_all, because axum sets an Allow of its own on a 405
        /// and a handler that appended rather than replaced would leave two.
        fn allow(&self) -> Vec<String> {
            let values: Vec<&str> = self
                .headers
                .get_all("allow")
                .iter()
                .map(|v| v.to_str().unwrap())
                .collect();
            assert!(values.len() <= 1, "one Allow header, got {values:?}");
            let mut m: Vec<String> = values
                .first()
                .copied()
                .unwrap_or_default()
                .split(',')
                .map(|s| s.trim().to_string())
                .filter(|s| !s.is_empty())
                .collect();
            m.sort();
            m
        }
        fn ids(&self) -> Vec<String> {
            self.json()
                .as_array()
                .expect("array")
                .iter()
                .map(|r| r["id"].as_str().unwrap().to_string())
                .collect()
        }
    }

    async fn call(
        app: &axum::Router,
        method: &str,
        uri: &str,
        actor: Option<&str>,
        body: Option<&str>,
    ) -> Res {
        let mut req = Request::builder().method(method).uri(uri);
        if let Some(a) = actor {
            req = req.header("X-User-ID", a);
        }
        let req = match body {
            Some(b) => req.header("content-type", "application/json").body(Body::from(b.to_string())),
            None => req.body(Body::empty()),
        }
        .unwrap();
        let res = app.clone().oneshot(req).await.unwrap();
        let status = res.status();
        let headers = res.headers().clone();
        let bytes = axum::body::to_bytes(res.into_body(), usize::MAX).await.unwrap();
        Res {
            status,
            headers,
            body: String::from_utf8(bytes.to_vec()).unwrap(),
        }
    }

    // ---- the three the brief pins -------------------------------------

    #[tokio::test]
    async fn unauthenticated_is_json_401() {
        let res = app().await.oneshot(
            Request::builder().uri("/api/me").body(Body::empty()).unwrap()
        ).await.unwrap();
        assert_eq!(res.status(), StatusCode::UNAUTHORIZED);
        let bytes = axum::body::to_bytes(res.into_body(), usize::MAX).await.unwrap();
        assert_eq!(&bytes[..], br#"{"error":"unauthorized"}"#);
    }

    #[tokio::test]
    async fn method_mismatch_is_405_with_allow() {
        let res = app().await.oneshot(
            Request::builder().uri("/api/login").body(Body::empty()).unwrap()
        ).await.unwrap();
        assert_eq!(res.status(), StatusCode::METHOD_NOT_ALLOWED);
        assert!(res.headers().get("allow").is_some(), "405 must carry an Allow header");
    }

    #[tokio::test]
    async fn trailing_slash_is_404_not_a_redirect() {
        let res = app().await.oneshot(
            Request::builder().uri("/api/me/").header("X-User-ID", "u_ada").body(Body::empty()).unwrap()
        ).await.unwrap();
        assert_eq!(res.status(), StatusCode::NOT_FOUND);
    }

    // ---- HEAD, OPTIONS and Allow --------------------------------------

    #[tokio::test]
    async fn head_on_a_get_route_is_405_and_carries_no_body() {
        let app = app().await;
        // Positive control: the GET this HEAD would otherwise fall through to.
        let get = call(&app, "GET", "/api/health", None, None).await;
        assert_eq!(get.status, StatusCode::OK);
        assert_eq!(get.body, r#"{"status":"ok"}"#);

        let head = call(&app, "HEAD", "/api/health", None, None).await;
        assert_eq!(head.status, StatusCode::METHOD_NOT_ALLOWED);
        assert_eq!(head.allow(), vec!["GET"]);
        assert_eq!(head.body, "", "RFC 9110 forbids a body on a HEAD response");
        assert_eq!(head.header("content-type"), None);
    }

    #[tokio::test]
    async fn options_is_405_with_the_same_body_as_any_other_mismatch() {
        let app = app().await;
        let opt = call(&app, "OPTIONS", "/api/me", Some("u_ada"), None).await;
        let put = call(&app, "PUT", "/api/me", Some("u_ada"), None).await;
        assert_eq!(opt.status, StatusCode::METHOD_NOT_ALLOWED);
        assert_eq!(opt.body, r#"{"error":"method not allowed"}"#);
        assert_eq!(opt.body, put.body);
        assert_eq!(opt.allow(), vec!["GET"]);
    }

    #[tokio::test]
    async fn allow_lists_the_contract_methods_and_nothing_else() {
        let app = app().await;
        // A two-method path is the one that can catch a hardcoded single
        // method, and every list must exclude HEAD and OPTIONS.
        for (uri, method, want) in [
            ("/api/members", "DELETE", vec!["GET", "POST"]),
            ("/api/resources", "PUT", vec!["GET", "POST"]),
            ("/api/resources/r_acme_pg/requests", "DELETE", vec!["GET", "POST"]),
            ("/api/requests/req_acme_1/comments", "PUT", vec!["GET", "POST"]),
            ("/api/login", "GET", vec!["POST"]),
            ("/api/logout", "GET", vec!["POST"]),
            ("/api/me", "POST", vec!["GET"]),
            ("/api/health", "POST", vec!["GET"]),
            ("/api/members/u_linus", "GET", vec!["DELETE"]),
            ("/api/members/u_linus/role", "POST", vec!["PUT"]),
            ("/api/resources/r_acme_pg/decommission", "GET", vec!["POST"]),
            ("/api/requests/req_acme_1/approve", "GET", vec!["POST"]),
            ("/api/requests/req_acme_1/deny", "GET", vec!["POST"]),
            ("/api/requests/req_acme_1/revoke", "GET", vec!["POST"]),
        ] {
            let res = call(&app, method, uri, Some("u_ada"), None).await;
            assert_eq!(res.status, StatusCode::METHOD_NOT_ALLOWED, "{method} {uri}");
            assert_eq!(res.allow(), want, "{method} {uri}");
        }
    }

    // ---- check precedence ---------------------------------------------

    #[tokio::test]
    async fn the_admin_check_runs_before_the_body_is_read() {
        let app = app().await;
        // Positive control: the same unparseable body from an admin is the
        // 400 the member never reaches.
        let admin = call(&app, "POST", "/api/resources", Some("u_ada"), Some("not json")).await;
        assert_eq!(admin.status, StatusCode::BAD_REQUEST);

        let member = call(&app, "POST", "/api/resources", Some("u_linus"), Some("not json")).await;
        assert_eq!(member.status, StatusCode::FORBIDDEN);
        assert_eq!(member.json()["error"], "admin role required");
    }

    #[tokio::test]
    async fn the_body_is_read_before_the_store_on_a_route_with_no_admin_check() {
        let app = app().await;
        // Positive control: a good body against the same unknown resource
        // is the 404 the malformed one never reaches.
        let ok = call(&app, "POST", "/api/resources/r_nope/requests", Some("u_linus"), Some(r#"{"reason":"why"}"#)).await;
        assert_eq!(ok.status, StatusCode::NOT_FOUND);

        let bad = call(&app, "POST", "/api/resources/r_nope/requests", Some("u_linus"), Some("not json")).await;
        assert_eq!(bad.status, StatusCode::BAD_REQUEST);
    }

    #[tokio::test]
    async fn authentication_runs_before_the_admin_and_body_checks() {
        let app = app().await;
        let res = call(&app, "POST", "/api/resources", None, Some("not json")).await;
        assert_eq!(res.status, StatusCode::UNAUTHORIZED);
        let unknown = call(&app, "POST", "/api/resources", Some("u_nobody"), Some("not json")).await;
        assert_eq!(unknown.status, StatusCode::UNAUTHORIZED);
    }

    // ---- the tenant boundary ------------------------------------------

    #[tokio::test]
    async fn a_cross_tenant_target_is_the_same_404_as_an_unregistered_path() {
        let app = app().await;
        // Positive control first: the same path from inside the tenant is a
        // 200, so the 404 below is about the boundary and not about the path.
        let own = call(&app, "GET", "/api/resources/r_acme_pg/requests", Some("u_ada"), None).await;
        assert_eq!(own.status, StatusCode::OK);
        assert_eq!(own.ids(), vec!["req_acme_1", "req_acme_4", "req_acme_5"]);

        // u_grace is an admin, so this is the tenant check and not the role check.
        let other = call(&app, "GET", "/api/resources/r_acme_pg/requests", Some("u_grace"), None).await;
        let unknown = call(&app, "GET", "/api/no-such-route", Some("u_grace"), None).await;
        assert_eq!(other.status, StatusCode::NOT_FOUND);
        assert_eq!(other.body, unknown.body);
        assert_eq!(other.body, r#"{"error":"not found"}"#);
    }

    #[tokio::test]
    async fn paths_are_case_sensitive_and_the_bare_mount_is_a_json_404() {
        let app = app().await;
        let ok = call(&app, "GET", "/api/me", Some("u_ada"), None).await;
        assert_eq!(ok.status, StatusCode::OK);
        assert_eq!(ok.json()["id"], "u_ada");

        for uri in ["/api/ME", "/api", "/api/", "/api/me/"] {
            let res = call(&app, "GET", uri, Some("u_ada"), None).await;
            assert_eq!(res.status, StatusCode::NOT_FOUND, "{uri}");
            assert_eq!(res.body, r#"{"error":"not found"}"#, "{uri}");
        }
    }

    // ---- bodies: trimming, nulls, unknown fields -----------------------

    #[tokio::test]
    async fn strings_are_trimmed_and_an_empty_sensitivity_defaults() {
        let app = app().await;
        let res = call(&app, "POST", "/api/resources", Some("u_ada"),
            Some(r#"{"name":"  vault  ","description":"  secrets  ","sensitivity":"   ","extra":1}"#)).await;
        assert_eq!(res.status, StatusCode::CREATED);
        let j = res.json();
        assert_eq!(j["name"], "vault");
        assert_eq!(j["description"], "secrets");
        assert_eq!(j["sensitivity"], "internal");
        assert_eq!(j["registered_by"], "u_ada");
        assert_eq!(j["organization_id"], "org_acme");

        // A name that trims to empty is the route's own 400.
        let blank = call(&app, "POST", "/api/resources", Some("u_ada"), Some(r#"{"name":"   "}"#)).await;
        assert_eq!(blank.status, StatusCode::BAD_REQUEST);
        assert_eq!(blank.json()["error"], "name required");
    }

    #[tokio::test]
    async fn null_is_a_missing_field_and_a_wrong_type_is_a_400() {
        let app = app().await;
        // Null on an optional field takes the default.
        let ok = call(&app, "POST", "/api/members", Some("u_ada"),
            Some(r#"{"name":"  Bob  ","email":"  bob@acme.test  ","role":null,"bogus":1}"#)).await;
        assert_eq!(ok.status, StatusCode::CREATED);
        assert_eq!(ok.json()["role"], "member");
        assert_eq!(ok.json()["name"], "Bob");
        assert_eq!(ok.json()["email"], "bob@acme.test");

        // Null on a required field is the route's own message, not a type error.
        let null_name = call(&app, "POST", "/api/members", Some("u_ada"),
            Some(r#"{"name":null,"email":"x@acme.test"}"#)).await;
        assert_eq!(null_name.status, StatusCode::BAD_REQUEST);
        assert_eq!(null_name.json()["error"], "name and email required");

        // Any other wrong type is a 400 too.
        let wrong = call(&app, "POST", "/api/members", Some("u_ada"),
            Some(r#"{"name":7,"email":"x@acme.test"}"#)).await;
        assert_eq!(wrong.status, StatusCode::BAD_REQUEST);

        // A missing body is a 400, not a panic.
        let empty = call(&app, "POST", "/api/members", Some("u_ada"), None).await;
        assert_eq!(empty.status, StatusCode::BAD_REQUEST);
    }

    #[tokio::test]
    async fn login_matches_the_email_exactly() {
        let app = app().await;
        let ok = call(&app, "POST", "/api/login", None, Some(r#"{"email":"ada@acme.test"}"#)).await;
        assert_eq!(ok.status, StatusCode::OK);
        assert_eq!(ok.json()["id"], "u_ada");

        let padded = call(&app, "POST", "/api/login", None, Some(r#"{"email":" ada@acme.test "}"#)).await;
        assert_eq!(padded.status, StatusCode::UNAUTHORIZED);
        assert_eq!(padded.json()["error"], "no user with that email");

        let blank = call(&app, "POST", "/api/login", None, Some(r#"{"email":""}"#)).await;
        assert_eq!(blank.status, StatusCode::BAD_REQUEST);
        assert_eq!(blank.json()["error"], "email required");
    }

    #[tokio::test]
    async fn a_role_change_compares_the_role_exactly() {
        let app = app().await;
        let ok = call(&app, "PUT", "/api/members/u_linus/role", Some("u_ada"), Some(r#"{"role":"admin"}"#)).await;
        assert_eq!(ok.status, StatusCode::OK);
        assert_eq!(ok.json()["role"], "admin");

        for body in [r#"{"role":" admin "}"#, r#"{"role":"owner"}"#, r#"{"role":null}"#, "{}"] {
            let res = call(&app, "PUT", "/api/members/u_linus/role", Some("u_ada"), Some(body)).await;
            assert_eq!(res.status, StatusCode::BAD_REQUEST, "{body}");
            assert_eq!(res.json()["error"], "role must be admin or member", "{body}");
        }
    }

    // ---- state transitions --------------------------------------------

    #[tokio::test]
    async fn a_decided_request_cannot_be_decided_again() {
        let app = app().await;
        let approved = call(&app, "POST", "/api/requests/req_acme_1/approve", Some("u_ada"), None).await;
        assert_eq!(approved.status, StatusCode::OK);
        assert_eq!(approved.json()["status"], "approved");
        assert_eq!(approved.json()["decided_by"], "u_ada");

        let again = call(&app, "POST", "/api/requests/req_acme_1/approve", Some("u_ada"), None).await;
        assert_eq!(again.status, StatusCode::CONFLICT);
        assert_eq!(again.json()["error"], "conflict");

        // Revoke keeps the approval it undoes, which only the fixture documents.
        let revoked = call(&app, "POST", "/api/requests/req_acme_1/revoke", Some("u_ada"), None).await;
        assert_eq!(revoked.status, StatusCode::OK);
        assert_eq!(revoked.json()["status"], "revoked");
        assert_eq!(revoked.json()["decided_by"], "u_ada");
        assert!(revoked.json()["decided_at"].is_string());
        assert_eq!(revoked.json()["revoked_by"], "u_ada");
    }

    #[tokio::test]
    async fn a_decommissioned_resource_takes_no_new_requests() {
        let app = app().await;
        let before = call(&app, "POST", "/api/resources/r_acme_pg/requests", Some("u_linus"), Some(r#"{"reason":"  debug  "}"#)).await;
        assert_eq!(before.status, StatusCode::CREATED);
        assert_eq!(before.json()["reason"], "debug");
        assert_eq!(before.json()["status"], "pending");

        let gone = call(&app, "POST", "/api/resources/r_acme_pg/decommission", Some("u_ada"), None).await;
        assert_eq!(gone.status, StatusCode::OK);

        let after = call(&app, "POST", "/api/resources/r_acme_pg/requests", Some("u_linus"), Some(r#"{"reason":"debug"}"#)).await;
        assert_eq!(after.status, StatusCode::CONFLICT);
    }

    #[tokio::test]
    async fn a_second_decommission_restamps_rather_than_conflicting() {
        let app = app().await;
        let first = call(&app, "POST", "/api/resources/r_acme_pg/decommission", Some("u_ada"), None).await;
        let second = call(&app, "POST", "/api/resources/r_acme_pg/decommission", Some("u_ada"), None).await;
        assert_eq!(second.status, StatusCode::OK);
        // Different stamps, which only survives sub-second precision.
        assert_ne!(first.json()["decommission_at"], second.json()["decommission_at"]);
    }

    // ---- content type --------------------------------------------------

    #[tokio::test]
    async fn every_body_is_json_and_the_204s_have_none() {
        let app = app().await;
        for (method, uri, actor, body) in [
            ("GET", "/api/health", None, None),
            ("GET", "/api/me", Some("u_ada"), None),
            ("GET", "/api/no-such-route", Some("u_ada"), None),
            ("POST", "/api/login", None, Some(r#"{"email":"nobody@acme.test"}"#)),
            ("GET", "/api/members", Some("u_linus"), None),
        ] {
            let res = call(&app, method, uri, actor, body).await;
            assert_eq!(
                res.header("content-type").as_deref(),
                Some("application/json; charset=utf-8"),
                "{method} {uri}"
            );
        }

        let out = call(&app, "POST", "/api/logout", Some("u_ada"), None).await;
        assert_eq!(out.status, StatusCode::NO_CONTENT);
        assert_eq!(out.header("content-type"), None);
        assert_eq!(out.body, "");

        let removed = call(&app, "DELETE", "/api/members/u_linus", Some("u_ada"), None).await;
        assert_eq!(removed.status, StatusCode::NO_CONTENT);
        assert_eq!(removed.header("content-type"), None);
        assert_eq!(removed.body, "");
    }
}
