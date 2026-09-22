mod handlers;
mod memory;
mod models;
mod router;
mod seed;
mod store;

use std::net::SocketAddr;
use std::path::PathBuf;
use std::sync::Arc;

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

    // Everything outside /api is the SPA. A path with no file behind it
    // still gets the API's JSON 404 rather than the static server's empty
    // one, so /API/me answers the same way /api/no-such-route does without
    // ever reaching the API.
    let spa = ServeDir::new(static_dir()).not_found_service(axum::routing::any(handlers::not_found));
    let app = router::build_router(store).fallback_service(spa);

    let addr = SocketAddr::from(([0, 0, 0, 0], port()));
    let listener = tokio::net::TcpListener::bind(addr)
        .await
        .unwrap_or_else(|e| panic!("bind {addr}: {e}"));
    println!("listening on http://{addr}");
    axum::serve(listener, app).await.expect("serve");
}
