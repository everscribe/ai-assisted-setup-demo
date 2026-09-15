# Contract log, Python store

Questions `contract/openapi.yaml` and `contract/seed.json` could not answer while
writing `app/models.py`, `app/store.py`, `app/seed.py` and `app/memory.py`, and
what I decided instead. Written as the work happened.

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
