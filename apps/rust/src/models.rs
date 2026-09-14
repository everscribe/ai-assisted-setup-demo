//! Domain records. Every field name, type and optionality comes from the
//! response schemas in contract/openapi.yaml.

use chrono::{DateTime, SecondsFormat, Utc};
use serde::{Serialize, Serializer};

/// RFC 3339 in UTC with a literal Z. AutoSi drops the fraction when the
/// instant is whole-second, which is what the seeded rows need, and keeps
/// every non-zero sub-second digit on a runtime-minted one.
fn rfc3339<S: Serializer>(t: &DateTime<Utc>, s: S) -> Result<S::Ok, S::Error> {
    s.serialize_str(&t.to_rfc3339_opts(SecondsFormat::AutoSi, true))
}

fn rfc3339_opt<S: Serializer>(t: &Option<DateTime<Utc>>, s: S) -> Result<S::Ok, S::Error> {
    match t {
        Some(t) => rfc3339(t, s),
        None => s.serialize_none(),
    }
}

#[derive(Debug, Clone, Serialize)]
pub struct Organization {
    pub id: String,
    pub name: String,
    #[serde(serialize_with = "rfc3339")]
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize)]
pub struct User {
    pub id: String,
    pub name: String,
    pub email: String,
    pub role: String,
    pub organization_id: String,
    #[serde(serialize_with = "rfc3339")]
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize)]
pub struct Resource {
    pub id: String,
    pub name: String,
    pub description: String,
    pub sensitivity: String,
    pub organization_id: String,
    pub registered_by: String,
    #[serde(serialize_with = "rfc3339")]
    pub created_at: DateTime<Utc>,
    #[serde(serialize_with = "rfc3339_opt", skip_serializing_if = "Option::is_none")]
    pub decommission_at: Option<DateTime<Utc>>,
}

#[derive(Debug, Clone, Serialize)]
pub struct AccessRequest {
    pub id: String,
    pub resource_id: String,
    pub organization_id: String,
    pub requested_by: String,
    pub reason: String,
    pub status: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub decided_by: Option<String>,
    #[serde(serialize_with = "rfc3339_opt", skip_serializing_if = "Option::is_none")]
    pub decided_at: Option<DateTime<Utc>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub revoked_by: Option<String>,
    #[serde(serialize_with = "rfc3339_opt", skip_serializing_if = "Option::is_none")]
    pub revoked_at: Option<DateTime<Utc>>,
    #[serde(serialize_with = "rfc3339")]
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize)]
pub struct Comment {
    pub id: String,
    pub request_id: String,
    pub organization_id: String,
    pub author_id: String,
    pub body: String,
    #[serde(serialize_with = "rfc3339")]
    pub created_at: DateTime<Utc>,
}
