"""Router and handler behaviour, driven through httpx.ASGITransport so no port
is bound. Every expectation comes from contract/openapi.yaml.
"""

import httpx
import pytest

from app.memory import MemoryStore
from app.router import build_app

# The contract's content type for every response that has a body.
JSON_CT = "application/json; charset=utf-8"

ADMIN = {"X-User-ID": "u_ada"}
MEMBER = {"X-User-ID": "u_linus"}

# Every admin-gated route: name, method, path, body, and the status an admin
# gets. A member must be refused on all of them.
ADMIN_ROUTES = [
    ("invite_member", "POST", "/api/members",
     b'{"name":"New P","email":"np@acme.test"}', 201),
    ("remove_member", "DELETE", "/api/members/u_linus", None, 204),
    ("change_member_role", "PUT", "/api/members/u_linus/role",
     b'{"role":"admin"}', 200),
    ("register_resource", "POST", "/api/resources", b'{"name":"vault"}', 201),
    ("decommission_resource", "POST", "/api/resources/r_acme_pg/decommission",
     None, 200),
    ("approve_request", "POST", "/api/requests/req_acme_1/approve", None, 200),
    ("deny_request", "POST", "/api/requests/req_acme_1/deny", None, 200),
    ("revoke_request", "POST", "/api/requests/req_acme_2/revoke", None, 200),
]

# Every route that reads a body: name, method, path, and whether an admin gate
# runs ahead of the parse. login is absent on purpose: it is anonymous, so it
# has no earlier check to run ahead of the parse.
BODY_ROUTES = [
    ("invite_member", "POST", "/api/members", True),
    ("change_member_role", "PUT", "/api/members/u_linus/role", True),
    ("register_resource", "POST", "/api/resources", True),
    ("request_access", "POST", "/api/resources/r_acme_pg/requests", False),
    ("add_comment", "POST", "/api/requests/req_acme_1/comments", False),
]


@pytest.fixture
def anyio_backend():
    return "asyncio"


@pytest.fixture
def client():
    store = MemoryStore()
    store.seed()
    app = build_app(store)
    transport = httpx.ASGITransport(app=app)
    return httpx.AsyncClient(transport=transport, base_url="http://t")


@pytest.mark.anyio
async def test_unauthenticated_is_json_401(client):
    r = await client.get("/api/me")
    assert r.status_code == 401
    assert r.json() == {"error": "unauthorized"}


@pytest.mark.anyio
async def test_head_health_is_405_not_an_empty_200(client):
    r = await client.request("HEAD", "/api/health")
    assert r.status_code == 405
    # openapi.yaml: a HEAD response carries the same headers the equivalent GET
    # would, Content-Type included, and that type is always charset-qualified.
    assert r.headers["content-type"] == JSON_CT
    assert "HEAD" not in r.headers["allow"].upper().split(", ")


@pytest.mark.anyio
async def test_trailing_slash_is_404_not_a_redirect(client):
    r = await client.get("/api/me/", headers={"X-User-ID": "u_ada"})
    assert r.status_code == 404
    assert r.json() == {"error": "not found"}


@pytest.mark.anyio
@pytest.mark.parametrize("name,method,path,admin_only", BODY_ROUTES,
                         ids=[row[0] for row in BODY_ROUTES])
async def test_every_body_route_checks_auth_and_role_before_parsing(
    client, name, method, path, admin_only
):
    # 401 first, then 403, then the parse. The malformed body is what makes
    # each row discriminate: a valid body is a 403 whichever order the handler
    # runs, so only an unparseable one catches a role check that has slipped
    # behind the parse. That is the reorder FastAPI performs for free if a
    # Pydantic model is bound as a route parameter, which is why no route here
    # binds one.
    malformed = b"{"

    anon = await client.request(method, path, content=malformed)
    assert anon.status_code == 401, name

    refused = await client.request(method, path, headers=MEMBER, content=malformed)
    assert refused.status_code == (403 if admin_only else 400), name
    if admin_only:
        assert refused.json() == {"error": "admin role required"}, name

    # Positive control: the same body from an admin reaches the parse, so the
    # row above is about order and not a route that refuses everyone.
    allowed = await client.request(method, path, headers=ADMIN, content=malformed)
    assert allowed.status_code == 400, name


@pytest.mark.anyio
@pytest.mark.parametrize("path", ["/api//me", "/api/resources//", "/api//", "/api//me/"])
async def test_empty_path_segment_is_json_404(path, client):
    # No code of ours rejects these: no registered path carries an empty
    # segment and {id} never matches across a slash, so they fall through to
    # the JSON 404. This pins that the router keeps behaving that way, since a
    # framework that normalised the path would serve /api/me instead.
    r = await client.get(path, headers=ADMIN)
    assert r.status_code == 404
    assert r.json() == {"error": "not found"}


@pytest.mark.anyio
async def test_case_variant_field_name_is_an_unknown_field(client):
    r = await client.post("/api/login", content=b'{"Email":"ada@acme.test"}')
    assert r.status_code == 400
    assert r.json() == {"error": "email required"}


@pytest.mark.anyio
async def test_percent_encoded_route_path_is_not_decoded_before_routing(client):
    # Positive control first: the unencoded path serves Ada's record, so the
    # 404 below cannot be a handler that refuses everything.
    ok = await client.get("/api/me", headers={"X-User-ID": "u_ada"})
    assert ok.status_code == 200
    assert ok.json()["id"] == "u_ada"

    for path in ("/api/%6de", "/api/%72esources"):
        r = await client.get(path, headers={"X-User-ID": "u_ada"})
        assert r.status_code == 404, path
        assert r.json() == {"error": "not found"}, path


@pytest.mark.anyio
async def test_percent_encoded_path_parameter_is_decoded(client):
    plain = await client.get("/api/resources/r_acme_pg/requests",
                             headers={"X-User-ID": "u_ada"})
    assert plain.status_code == 200
    encoded = await client.get("/api/resources/r%5Facme_pg/requests",
                               headers={"X-User-ID": "u_ada"})
    assert encoded.status_code == 200
    assert encoded.json() == plain.json()


@pytest.mark.anyio
async def test_percent_encoded_mount_does_not_reach_the_api(client):
    # Only the status is pinned here: the body belongs to whatever serves the
    # SPA. What matters is that it is not Ada's record.
    r = await client.get("/%61pi/me", headers={"X-User-ID": "u_ada"})
    assert r.status_code == 404
    assert "u_ada" not in r.text


@pytest.mark.anyio
async def test_case_variant_route_path_does_not_reach_the_api(client):
    r = await client.get("/api/ME", headers={"X-User-ID": "u_ada"})
    assert r.status_code == 404
    assert r.json() == {"error": "not found"}


@pytest.mark.anyio
async def test_allow_lists_every_method_registered_for_the_path(client):
    r = await client.request("DELETE", "/api/members", headers={"X-User-ID": "u_ada"})
    assert r.status_code == 405
    assert r.json() == {"error": "method not allowed"}
    assert sorted(r.headers["allow"].split(", ")) == ["GET", "POST"]


@pytest.mark.anyio
async def test_options_is_a_405_with_the_json_body(client):
    r = await client.request("OPTIONS", "/api/health")
    assert r.status_code == 405
    assert r.json() == {"error": "method not allowed"}


@pytest.mark.anyio
async def test_cross_tenant_resource_is_404(client):
    # Positive control: the same route inside the actor's own org is a 200, so
    # the 404 below is the tenant boundary and not an unregistered path.
    ok = await client.get("/api/resources/r_acme_pg/requests",
                          headers={"X-User-ID": "u_ada"})
    assert ok.status_code == 200
    r = await client.get("/api/resources/r_globex_k8s/requests",
                         headers={"X-User-ID": "u_ada"})
    assert r.status_code == 404
    assert r.json() == {"error": "not found"}


@pytest.mark.anyio
async def test_body_is_read_before_the_store_on_a_route_with_no_admin_check(client):
    # Positive control: a valid body on a known resource is a 201.
    ok = await client.post("/api/resources/r_acme_pg/requests",
                           headers={"X-User-ID": "u_linus"},
                           content=b'{"reason":"Debug PAY-4821."}')
    assert ok.status_code == 201
    r = await client.post("/api/resources/r_nope/requests",
                          headers={"X-User-ID": "u_linus"}, content=b"{")
    assert r.status_code == 400


@pytest.mark.anyio
async def test_logout_204_carries_no_content_type(client):
    r = await client.post("/api/logout", headers={"X-User-ID": "u_ada"})
    assert r.status_code == 204
    assert "content-type" not in r.headers
    assert r.content == b""


@pytest.mark.anyio
async def test_trimming_and_defaults_on_register_resource(client):
    r = await client.post("/api/resources", headers=ADMIN,
                          content=b'{"name":"  billing-db  ","sensitivity":null}')
    assert r.status_code == 201
    body = r.json()
    assert body["name"] == "billing-db"
    assert body["sensitivity"] == "internal"
    assert body["description"] == ""

    padded = await client.post("/api/resources", headers=ADMIN,
                               content=b'{"name":"cache","description":"  hot keys  ",'
                                       b'"sensitivity":"  public  "}')
    assert padded.status_code == 201
    assert padded.json()["description"] == "hot keys"
    assert padded.json()["sensitivity"] == "public"

    blank = await client.post("/api/resources", headers=ADMIN,
                              content=b'{"name":"   "}')
    assert blank.status_code == 400
    assert blank.json() == {"error": "name required"}


@pytest.mark.anyio
async def test_role_is_compared_exactly(client):
    r = await client.put("/api/members/u_linus/role", headers={"X-User-ID": "u_ada"},
                         content=b'{"role":" admin "}')
    assert r.status_code == 400
    assert r.json() == {"error": "role must be admin or member"}
    still = await client.get("/api/members", headers={"X-User-ID": "u_ada"})
    assert [m["role"] for m in still.json() if m["id"] == "u_linus"] == ["member"]


@pytest.mark.anyio
@pytest.mark.parametrize("name,method,path,body,ok_status", ADMIN_ROUTES,
                         ids=[row[0] for row in ADMIN_ROUTES])
async def test_admin_role_is_required_on_every_admin_gated_route(
    client, name, method, path, body, ok_status
):
    refused = await client.request(method, path, headers=MEMBER, content=body)
    assert refused.status_code == 403, name
    assert refused.json() == {"error": "admin role required"}, name
    # Positive control: the same call as an admin must succeed, or a handler
    # that refuses everything would pass this for free.
    allowed = await client.request(method, path, headers=ADMIN, content=body)
    assert allowed.status_code == ok_status, name


@pytest.mark.anyio
async def test_an_unknown_actor_is_401(client):
    # Positive control: a known actor is a 200 on the same route.
    ok = await client.get("/api/me", headers=ADMIN)
    assert ok.status_code == 200
    r = await client.get("/api/me", headers={"X-User-ID": "u_nobody"})
    assert r.status_code == 401
    assert r.json() == {"error": "unauthorized"}


@pytest.mark.anyio
async def test_each_decision_verb_drives_its_own_transition(client):
    approved = await client.post("/api/requests/req_acme_1/approve", headers=ADMIN)
    assert approved.status_code == 200
    assert approved.json()["status"] == "approved"

    revoked = await client.post("/api/requests/req_acme_1/revoke", headers=ADMIN)
    assert revoked.status_code == 200
    assert revoked.json()["status"] == "revoked"
    # Revoking leaves the approval's decided_by alone.
    assert revoked.json()["decided_by"] == "u_ada"

    fresh = await client.post("/api/resources/r_acme_pg/requests", headers=MEMBER,
                              content=b'{"reason":"Verify the replica lag alert."}')
    assert fresh.status_code == 201
    request_id = fresh.json()["id"]
    denied = await client.post(f"/api/requests/{request_id}/deny", headers=ADMIN)
    assert denied.status_code == 200
    assert denied.json()["status"] == "denied"


@pytest.mark.anyio
async def test_a_transition_that_does_not_apply_is_409_conflict(client):
    # Positive control: the first deny is a 200, so the 409 below is the state
    # machine and not a handler that refuses everything.
    first = await client.post("/api/requests/req_acme_1/deny", headers=ADMIN)
    assert first.status_code == 200
    again = await client.post("/api/requests/req_acme_1/deny", headers=ADMIN)
    assert again.status_code == 409
    assert again.json() == {"error": "conflict"}


@pytest.mark.anyio
async def test_member_name_and_email_are_trimmed(client):
    r = await client.post("/api/members", headers=ADMIN,
                          content=b'{"name":"  Grace H  ","email":"  gh@acme.test  "}')
    assert r.status_code == 201
    assert r.json()["name"] == "Grace H"
    assert r.json()["email"] == "gh@acme.test"

    blank = await client.post("/api/members", headers=ADMIN,
                              content=b'{"name":"   ","email":"   "}')
    assert blank.status_code == 400
    assert blank.json() == {"error": "name and email required"}


@pytest.mark.anyio
async def test_access_request_reason_is_trimmed(client):
    r = await client.post("/api/resources/r_acme_pg/requests", headers=MEMBER,
                          content=b'{"reason":"  Debug PAY-4821.  "}')
    assert r.status_code == 201
    assert r.json()["reason"] == "Debug PAY-4821."

    blank = await client.post("/api/resources/r_acme_pg/requests", headers=MEMBER,
                              content=b'{"reason":"   "}')
    assert blank.status_code == 400
    assert blank.json() == {"error": "reason required"}


@pytest.mark.anyio
async def test_comment_body_is_trimmed(client):
    r = await client.post("/api/requests/req_acme_1/comments", headers=MEMBER,
                          content=b'{"body":"  Scope is read-only.  "}')
    assert r.status_code == 201
    assert r.json()["body"] == "Scope is read-only."

    blank = await client.post("/api/requests/req_acme_1/comments", headers=MEMBER,
                              content=b'{"body":"   "}')
    assert blank.status_code == 400
    assert blank.json() == {"error": "body required"}


@pytest.mark.anyio
async def test_login_email_is_pointedly_not_trimmed(client):
    # Positive control: the exact address logs in.
    ok = await client.post("/api/login", content=b'{"email":"ada@acme.test"}')
    assert ok.status_code == 200
    assert ok.json()["id"] == "u_ada"

    padded = await client.post("/api/login", content=b'{"email":" ada@acme.test "}')
    assert padded.status_code == 401
    assert padded.json() == {"error": "no user with that email"}
