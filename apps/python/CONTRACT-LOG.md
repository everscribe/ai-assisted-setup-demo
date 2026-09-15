# Contract log, Python store

Questions `contract/openapi.yaml` and `contract/seed.json` could not answer while
writing `app/models.py`, `app/store.py`, `app/seed.py`, `app/memory.py`,
`app/handlers.py`, `app/router.py` and `app/main.py`, and what I decided instead.
Written as the work happened.

I did not open `apps/go/handlers/`, `apps/go/store/memory/`,
`apps/node/src/handlers.ts`, `apps/node/src/memory.ts`, `apps/rust/src/handlers.rs`
or `apps/rust/src/memory.rs`. The only app file I read was `apps/rust/src/store.rs`,
which the brief permits for the trait shape, plus directory listings of the other
three apps.

This is the fourth port, against a contract three implementations have already
stress-tested, so the list is short on purpose. It is not padded.

---

## 1. How long is the random hex in a generated id?

`x-ordering` says generated ids carry "a random hex suffix" and the prefixes are
fixed (`u_`, `r_`, `req_`, `c_`), but no length is given, and nothing in the
contract can depend on one: ids are opaque to every conformance check and the
total order never falls back to id alone across apps.

Decided: `secrets.token_hex(6)`, twelve hex characters. Any length conforms.

## 2. Which layer applies the defaults, the handler or the store?

The contract fixes the observable result but not where it happens. An omitted,
null or empty `sensitivity` becomes `internal`, and a `role` that is omitted,
null or not one of `admin`/`member` becomes `member`. Both rules sit in the
request-schema prose, which is handler territory, yet the store is the thing
that writes the row.

Decided: the store also normalizes. Empty `sensitivity` becomes `internal` and a
`role` outside `admin`/`member` becomes `member` on `invite_member`. The handler
in task 3 owns trimming and the 400s, which the store cannot do, since by then a
trimmed-to-empty `name` is indistinguishable from one that was never sent.
Normalizing in both places is conformant either way and neither can be the
sole source of a wrong value.

Note the asymmetry the contract does pin: `update_member_role` compares `role`
exactly, so `" admin "` is a 400 rather than a promotion. The store therefore
does NOT normalize there, it assigns whatever it is handed, and rejecting the
value is the handler's job.

## 3. Does `seed()` replace or append?

No route calls it and the contract never describes calling it twice.

Decided: it replaces every collection, so seeding twice leaves the fixture
rather than two of everything.

## 4. Can an admin remove themselves?

`DELETE /api/members/{id}` is admin only and lists no 409. The prose says the
removed id stops authenticating immediately, which reads as though self-removal
is a real case, but it never says so.

Decided: allowed. No self-removal check, because inventing a 409 the route does
not document would diverge from a port that read the same page.

## 5. What order does `list_organizations` return?

`x-ordering` covers the four list endpoints and organizations have none, because
no route exposes them.

Decided: the same total order as everything else, `created_at` ascending then id
ascending. Unobservable either way.

## 6. The brief's HEAD fixture asserted a Content-Type the contract forbids

The task brief hands the router tests over verbatim, and one line reads
`assert r.headers["content-type"] == "application/json"`. The contract's last
paragraph before `servers:` says every response that has a body is
`application/json; charset=utf-8`, and the HEAD paragraph says a HEAD response
carries the same header fields the equivalent GET would, Content-Type included.
The two cannot both hold.

Decided: the contract wins. The app emits `application/json; charset=utf-8`
everywhere, via a `JSONResponse` subclass, because Starlette only appends a
charset to `text/*` media types and its bare `JSONResponse` would have shipped
`application/json`. The fixture now asserts the charset-qualified value. Nothing
in a single-app conformance run checks this: `TestAppsAgreeWithEachOther`, which
compares Content-Type across apps, skips below two apps, so the assertion in
`tests/test_router.py` is the only thing holding it.

## 7. What answers a path that is not under /api?

The percent-encoding rule pins only the status for `GET /%61pi/me`, saying the
body "falls outside the API entirely, onto whatever serves the SPA, whose 404
body is not the contract's to specify". The empty-segment rule is scoped the
same way, to paths "under /api". So nothing says what a request above the mount
should look like, and `web/dist` does not exist until task 4.

Decided: anything not byte-exactly under `/api` goes to `StaticFiles`, and while
`web/dist` is missing it answers `{"error": "not found"}` with a 404 rather than
constructing a placeholder dist. That keeps the status the contract pins, keeps
one body shape across the whole server, and leaks nothing: `/%61pi/me`,
`/API/me`, `//api/me` and `/nope` are all the same 404.

## 8. Which message accompanies a 400 that is not one of the six quoted ones?

The contract names six stable messages (`email required`, `name required`,
`name and email required`, `reason required`, `body required`,
`role must be admin or member`) and says the rest of the 400 text is not
normative, only that no check may match a message beginning `invalid body:`.
That leaves an unparseable body, a non-object body, and a field of the wrong
type without a message.

Decided: `{"error": "invalid body"}` for all three, with one exception. A
wrong-typed `role` on `PUT /api/members/{id}/role` answers
`role must be admin or member`, because that route's message already covers
every value that is not exactly one of the two strings and splitting it by
JSON type would be a distinction the contract never draws.

This is the one place this port went its own way. The task 3 review reports
that `{"role": 5}` answers `role must be admin or member` here and
`invalid body` on node and rust. All four are a 400, the contract says the
text is not normative, and the four-way conformance run passes, so it is not a
conformance failure. It is a divergence, and the log's job is to record it.

---

## Addendum, written in task 5 with all four apps in view

The read restriction is lifted for task 5, so these two are not questions the
contract could not answer; they are answers found by comparing four running
apps, recorded here because this file is where this port's contract questions
live.

### Item 6 is now checked across apps

The `application/json; charset=utf-8` rule was held for this port by
`tests/test_router.py` alone, because the only cross-app Content-Type check
skips below two apps. Four apps in compose means
`TestAppsAgreeWithEachOther` now compares that header on every lifecycle
step, and it agrees across go, node, rust and python.

### A dot segment was an open hole, and it was Go's

`GET /api/./me` and `GET /api/resources/../me` were a 404 on node, rust and
python and a 301 to `/api/me` with an HTML body on go, from net/http's
ServeMux cleaning the path. Same existence oracle the trailing-slash,
empty-segment and percent-encoding rules already close. The contract now names it,
go rejects dot segments alongside empty ones, and two lifecycle steps plus
`TestDotSegmentPathIsTheSame404AsAnUnknownPath` exercise it. This port needed
no change: routing on the raw path with no cleaning already gave the right
answer.

### Go's HTML escaping is accepted, not a divergence to fix

Go's `encoding/json` escapes `<`, `>`, `&` and U+2028 into backslash-u
forms; starlette's `JSONResponse` (`ensure_ascii=False`) emits them raw, as
do node and rust. Both spellings decode to the same string. The contract now
says so explicitly and a lifecycle step registers a resource carrying all
four characters, so what is checked is that the value survives, not how it is
spelled on the wire.
