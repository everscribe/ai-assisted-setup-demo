mod audit;
mod handlers;
mod memory;
mod models;
mod router;
mod seed;
mod store;

use std::net::SocketAddr;
use std::path::PathBuf;
use std::sync::Arc;

use axum::routing::post;
use everscribe::axum::EverscribeLayer;
use everscribe::minter::MinterOptions;
use everscribe::recorder::RecorderOptions;
use tower_http::services::ServeDir;

use crate::store::Store;

/// Where the SPA lives. The crate root is the working directory in dev and
/// in the container, so web/dist is the normal answer. The repo-root form
/// is honoured when it is the one that exists, and STATIC_DIR overrides
/// both for a layout that puts the files somewhere else.
fn static_dir() -> PathBuf {
    if let Ok(dir) = std::env::var("STATIC_DIR") {
        return dir.into();
    }
    let from_repo_root = PathBuf::from("apps/rust/web/dist");
    if from_repo_root.is_dir() {
        return from_repo_root;
    }
    "web/dist".into()
}

fn port() -> u16 {
    std::env::var("PORT")
        .ok()
        .and_then(|p| p.parse().ok())
        .unwrap_or(9003)
}

#[tokio::main]
async fn main() {
    let store: Arc<dyn Store> = Arc::new(memory::MemoryStore::new());
    store.seed().await.expect("seed the fixture");

    // Credentials come from EVERSCRIBE_PROJECT_ID and EVERSCRIBE_API_KEY.
    // A missing credential turns auditing off rather than killing the
    // process: this app has to stay runnable on a clean checkout, which is
    // what `cargo run` and the conformance harness depend on. The log line
    // is the only thing standing between that and a deploy that looks
    // healthy while silently recording nothing, so it says so plainly.
    let audit_parts = match everscribe::new_from_env() {
        Ok(es) => Some((
            es.new_recorder(RecorderOptions::default()),
            Arc::new(es.new_minter(MinterOptions::default())),
        )),
        Err(err) => {
            eprintln!("everscribe: audit logging DISABLED: {err}");
            None
        }
    };

    // Outside /api deliberately: this route is not in
    // contract/openapi.yaml, and the conformance harness compares the four
    // apps over the /api surface only. Merged before fallback_service,
    // because axum panics when merging two routers that both set one.
    let mut api = router::build_router(store.clone());
    if let Some((_, minter)) = &audit_parts {
        api = api.merge(
            axum::Router::new()
                .route("/everscribe/embed-token", post(audit::embed_token))
                .with_state((store.clone(), minter.clone())),
        );
    }

    // Everything outside /api is the SPA. A path with no file behind it
    // still gets the API's JSON 404 rather than the static server's empty
    // one, so /API/me answers the same way /api/no-such-route does without
    // ever reaching the API.
    let spa = ServeDir::new(static_dir()).not_found_service(axum::routing::any(handlers::not_found));
    let mut app = api.fallback_service(spa);

    if let Some((recorder, _)) = audit_parts {
        // Layer order is load-bearing and reads bottom-up: a layer added
        // later wraps the outside and runs first. attach_actor must
        // populate the request extensions before EverscribeLayer's
        // resolver reads them, so it is added last.
        app = app
            .layer(EverscribeLayer::new(recorder, audit::resolve_actor))
            .layer(axum::middleware::from_fn_with_state(
                store,
                audit::attach_actor,
            ));
    }

    let addr = SocketAddr::from(([0, 0, 0, 0], port()));
    let listener = tokio::net::TcpListener::bind(addr)
        .await
        .unwrap_or_else(|e| panic!("bind {addr}: {e}"));
    println!("listening on http://{addr}");
    axum::serve(listener, app).await.expect("serve");
}
