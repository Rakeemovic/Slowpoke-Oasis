import { defineConfig, devices } from "@playwright/test";

// The end-to-end tests run against their own backend and their own build of
// the web app, on ports apart from the usual 8080/8081. A running development
// setup and its database stay untouched.
const API_PORT = 9090;
const WEB_PORT = 8090;

export default defineConfig({
  testDir: "./e2e",
  // Every test registers its own users, so tests do not depend on each other.
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: `http://localhost:${WEB_PORT}`,
    // A fixed locale makes amounts predictable, e.g. "12,50 €".
    locale: "de-DE",
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    {
      // A fresh database in the temp directory on every run; "rm" also removes
      // SQLite's side files. The shell sets the path, so this file needs no
      // Node modules (the app's TypeScript setup has no Node types, on purpose).
      command: 'export DB_PATH="${TMPDIR:-/tmp}/slowpoke-e2e.db" && rm -f "$DB_PATH"* && go run ./cmd/server',
      cwd: "../backend",
      env: {
        ADDR: `:${API_PORT}`,
        CORS_ORIGINS: `http://localhost:${WEB_PORT}`,
      },
      url: `http://localhost:${API_PORT}/me`,
      // /me answers 401 without a session; Playwright counts 401 as "server is up".
      reuseExistingServer: false,
      timeout: 120_000,
    },
    {
      // EXPO_NO_DOTENV keeps frontend/.env.local from pointing the build at
      // the development API instead of the test backend.
      command: `npx expo export -p web --output-dir e2e/.dist && node e2e/serve.mjs e2e/.dist ${WEB_PORT}`,
      env: {
        EXPO_NO_DOTENV: "1",
        EXPO_PUBLIC_API_URL: `http://localhost:${API_PORT}`,
      },
      url: `http://localhost:${WEB_PORT}`,
      reuseExistingServer: false,
      timeout: 240_000,
    },
  ],
});
