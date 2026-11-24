import { defineConfig, devices } from "@playwright/test";

// The end-to-end tests run against their own backend, which also delivers a
// fresh export of the web app, just like a self-hosted server: app and API
// share one origin, the API lives under /api. The port is apart from the
// usual 8080/8081, so a running development setup and its database stay
// untouched.
const PORT = 9090;

export default defineConfig({
  testDir: "./e2e",
  // Every test registers its own users, so tests do not depend on each other.
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: `http://localhost:${PORT}`,
    // A fixed locale makes amounts predictable, e.g. "12,50 €".
    locale: "de-DE",
    // Counting numbers and fading rows would make texts change under the
    // tests' eyes; the app skips them when the system asks for less motion.
    reducedMotion: "reduce",
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    // First the export, then a fresh database in the temp directory on every
    // run; "rm" also removes SQLite's side files. The shell sets the paths, so
    // this file needs no Node modules (the app's TypeScript setup has no Node
    // types, on purpose). EXPO_NO_DOTENV keeps frontend/.env.local from
    // pointing the build at the development API.
    command:
      "npx expo export -p web --output-dir e2e/.dist && " +
      'export WEB_DIR="$PWD/e2e/.dist" DB_PATH="${TMPDIR:-/tmp}/slowpoke-e2e.db" && rm -f "$DB_PATH"* && ' +
      "cd ../backend && go run ./cmd/server",
    env: {
      ADDR: `:${PORT}`,
      EXPO_NO_DOTENV: "1",
      EXPO_PUBLIC_API_URL: "/api",
    },
    url: `http://localhost:${PORT}/api/me`,
    // /api/me answers 401 without a session; Playwright counts 401 as "server is up".
    reuseExistingServer: false,
    timeout: 240_000,
  },
});
