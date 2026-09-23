import { fileURLToPath, pathToFileURL } from "node:url";
import path from "node:path";
import express from "express";
import { createFromEnv } from "@everscribe/sdk-node";
import { expressMiddleware } from "@everscribe/sdk-node/express";
import type { Client as MinterClient } from "@everscribe/sdk-node/minter";
import type { Recorder } from "@everscribe/sdk-node/recorder";
import { MemoryStore } from "./memory.js";
import { attachActor, embedToken, resolveActor } from "./everscribe.js";
import { buildRouter } from "./router.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const webDist = path.join(__dirname, "..", "web", "dist");

/** Everscribe surfaces the server wires in. Optional so the router tests,
 *  which call makeApp() with no arguments, neither need credentials nor
 *  reach the network. */
export interface Audit {
  recorder: Recorder;
  minter: MinterClient;
}

// Seeds a store and mounts the router. Exported so tests get a fresh
// app without going through the network.
export async function makeApp(audit?: Audit): Promise<express.Express> {
  const store = new MemoryStore();
  await store.seed();

  const app = express();
  app.disable("x-powered-by");
  // The mount itself must be case sensitive, or /API/me reaches the API
  // router and answers as if the path existed. Same reason as the
  // router's own caseSensitive flag: a mixed-case probe must not be an
  // existence oracle.
  app.set("case sensitive routing", true);
  app.set("strict routing", true);

  if (audit) {
    // Order is load-bearing: attachActor populates req.actor, and the
    // audit middleware's resolveActor reads it. Reversed, every event
    // would record anonymously.
    app.use(attachActor(store));
    app.use(expressMiddleware({ recorder: audit.recorder, resolveActor }));

    // Outside /api deliberately: this route is not in
    // contract/openapi.yaml, and the conformance harness compares the
    // four apps over the /api surface only.
    app.post("/everscribe/embed-token", embedToken(audit.minter));
  }

  app.use("/api", buildRouter(store));
  app.use(express.static(webDist));

  return app;
}

// pathToFileURL, not string interpolation: a path containing a space
// or any other character that has to be percent-encoded would never
// match, and the process would start, listen on nothing, and exit 0.
const isMain =
  process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href;
/** Builds the Everscribe surfaces from the environment, or returns
 *  undefined when EVERSCRIBE_PROJECT_ID / EVERSCRIBE_API_KEY are absent.
 *
 *  A missing credential turns auditing off rather than killing the
 *  process: this app has to stay runnable on a clean checkout, which is
 *  what `pnpm run dev` and the conformance harness depend on. The warning
 *  is the only thing standing between that and a deploy that looks healthy
 *  while silently recording nothing, so it says so plainly. */
function auditFromEnv(): Audit | undefined {
  try {
    const es = createFromEnv();
    return { recorder: es.newRecorder(), minter: es.newMinter() };
  } catch (err) {
    console.warn(`everscribe: audit logging DISABLED: ${(err as Error).message}`);
    return undefined;
  }
}

if (isMain) {
  const port = Number(process.env.PORT ?? 9002);
  const app = await makeApp(auditFromEnv());
  app.listen(port, () => {
    console.log(`node app listening on ${port}`);
  });
}
