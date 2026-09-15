"""Router and handler behaviour, driven through httpx.ASGITransport so no port
is bound. Every expectation comes from contract/openapi.yaml.
"""

import httpx
import pytest

from app.memory import MemoryStore
from app.router import build_app

# The contract's content type for every response that has a body.
JSON_CT = "application/json; charset=utf-8"


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
async def test_admin_check_precedes_body_parsing(client):
    # A member sending garbage to an admin route gets 403, not 400.
    # Positive control: the same route with a valid body and an admin actor
    # must succeed, or a handler that refuses everything would pass this.
    ok = await client.post("/api/members", headers={"X-User-ID": "u_ada"},
                           content=b'{"name":"Grace H","email":"gh@acme.test"}')
    assert ok.status_code == 201
    r = await client.post("/api/members", headers={"X-User-ID": "u_linus"}, content=b"{")
    assert r.status_code == 403
    assert r.json() == {"error": "admin role required"}


@pytest.mark.anyio
async def test_empty_path_segment_is_json_404(client):
    r = await client.get("/api//me", headers={"X-User-ID": "u_ada"})
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
    r = await client.post("/api/resources", headers={"X-User-ID": "u_ada"},
                          content=b'{"name":"  billing-db  ","sensitivity":null}')
    assert r.status_code == 201
    body = r.json()
    assert body["name"] == "billing-db"
    assert body["sensitivity"] == "internal"
    assert body["description"] == ""

    blank = await client.post("/api/resources", headers={"X-User-ID": "u_ada"},
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
