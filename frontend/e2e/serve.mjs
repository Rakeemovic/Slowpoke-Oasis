// Serves the exported web app for the end-to-end tests.
// Usage: node e2e/serve.mjs <dir> <port>
//
// The app is a single-page app: paths like /ledgers/abc have no file of their
// own, so every unknown path gets index.html and the router takes over.
// Kept dependency-free on purpose; it only ever runs on localhost.
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { extname, join, normalize, resolve, sep } from "node:path";

const [dir, port] = process.argv.slice(2);
if (!dir || !port) {
  console.error("usage: node e2e/serve.mjs <dir> <port>");
  process.exit(1);
}
const root = resolve(dir);

const types = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json",
  ".png": "image/png",
  ".ico": "image/x-icon",
  ".svg": "image/svg+xml",
  ".ttf": "font/ttf",
};

async function load(path) {
  // normalize + the prefix check stop "../" from reading outside the build.
  const file = normalize(join(root, decodeURIComponent(path)));
  if (!file.startsWith(root + sep)) {
    return null;
  }
  try {
    return { body: await readFile(file), type: types[extname(file)] ?? "application/octet-stream" };
  } catch {
    return null;
  }
}

createServer(async (req, res) => {
  const path = new URL(req.url ?? "/", "http://localhost").pathname;
  const found = (path !== "/" && (await load(path))) || (await load("/index.html"));
  if (!found) {
    res.writeHead(500).end("index.html missing; was the app exported?");
    return;
  }
  res.writeHead(200, { "Content-Type": found.type }).end(found.body);
}).listen(Number(port), () => console.log(`serving ${root} on http://localhost:${port}`));
