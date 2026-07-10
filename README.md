# AI-assisted-setup-demo

This is a sample repo you can use to get a feel for how you can use
AI to setup Everscribe for a codebase either using the [Everscribe hosted AI agent](https://everscribe.io/docs/quickstart/hosted-ui) or the [BYOK flow](https://everscribe.io/docs/quickstart/skill).

You'll want to run it locally first to get an idea of what the example app
looks like before Everscribe is setup. Then once you go through the flow of your choice, you can run it again and to see how easy it was to to setup

## Running it locally

Requires Go 1.22+ and Node 18+.

```bash
make dev   # build the SPA and run the Go server on :9001
```

Open <http://localhost:9001> and sign in as one of the seeded users. For hot
reload, run the Vite dev server against the API instead:

```bash
go run .               # Go API on :9001
cd web && npm run dev  # Vite dev server, /api proxied
```

### With Docker

No Go or Node toolchain needed, just Docker:

```bash
docker build -t everscribe-demo .
docker run --rm -p 9001:9001 everscribe-demo
```

Then open <http://localhost:9001>.
