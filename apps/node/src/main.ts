import { fileURLToPath } from "node:url";
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
  app.use("/api", buildRouter(store));
  app.use(express.static(webDist));

  return app;
}

const isMain = process.argv[1] !== undefined && import.meta.url === `file://${process.argv[1]}`;
if (isMain) {
  const port = Number(process.env.PORT ?? 9002);
  const app = await makeApp();
  app.listen(port, () => {
    console.log(`node app listening on ${port}`);
  });
}
