import { fileURLToPath, pathToFileURL } from "node:url";
import path from "node:path";
import express from "express";
import { MemoryStore } from "./memory.js";
import { buildRouter } from "./router.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const webDist = path.join(__dirname, "..", "web", "dist");

// Seeds a store and mounts the router. Exported so tests get a fresh
// app without going through the network.
export async function makeApp(): Promise<express.Express> {
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
  app.use("/api", buildRouter(store));
  app.use(express.static(webDist));

  return app;
}

// pathToFileURL, not string interpolation: a path containing a space
// or any other character that has to be percent-encoded would never
// match, and the process would start, listen on nothing, and exit 0.
const isMain =
  process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href;
if (isMain) {
  const port = Number(process.env.PORT ?? 9002);
  const app = await makeApp();
  app.listen(port, () => {
    console.log(`node app listening on ${port}`);
  });
}
