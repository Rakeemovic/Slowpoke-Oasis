import { getToken, usesCookieSession } from "@/api/token";

// The API address comes from the build environment, so self-hosters can point
// the app at their own server (EXPO_PUBLIC_API_URL in frontend/.env).
const baseUrl = (process.env.EXPO_PUBLIC_API_URL ?? "http://localhost:8080").replace(/\/$/, "");

/** An error answer from the API, or status 0 when the server was not reached. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

type Method = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

let unauthorizedHandler: (() => void) | undefined;

/**
 * Registers a function that runs whenever the API answers 401, which means the
 * session expired or was ended elsewhere. The auth provider uses it to log out.
 */
export function setUnauthorizedHandler(handler: (() => void) | undefined) {
  unauthorizedHandler = handler;
}

/** Sends a request to the API and returns the decoded JSON answer. */
export async function request<T>(method: Method, path: string, body?: unknown): Promise<T> {
  const response = await send(method, path, body);
  if (response.status === 401) {
    unauthorizedHandler?.();
  }
  if (!response.ok) {
    throw new ApiError(response.status, await errorMessage(response));
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

async function send(method: Method, path: string, body: unknown): Promise<Response> {
  // Built outside the try, so a failing token store is not reported as offline.
  const requestHeaders = await headers(body !== undefined);
  try {
    return await fetch(baseUrl + path, {
      method,
      headers: requestHeaders,
      body: body === undefined ? undefined : JSON.stringify(body),
      // The browser only attaches the session cookie with "include".
      credentials: usesCookieSession ? "include" : "omit",
    });
  } catch (error) {
    if (__DEV__) {
      console.warn(`${method} ${baseUrl + path} failed:`, error);
    }
    // fetch only throws when no answer arrived at all (offline, wrong address, CORS).
    throw new ApiError(0, "Cannot reach the server. Please check your connection.");
  }
}

async function headers(hasBody: boolean): Promise<Record<string, string>> {
  const result: Record<string, string> = {};
  if (hasBody) {
    result["Content-Type"] = "application/json";
  }
  if (usesCookieSession) {
    // Required for cookie sessions as CSRF protection (rule A8).
    result["X-Requested-With"] = "slowpoke-oasis";
  } else {
    const token = await getToken();
    if (token) {
      result.Authorization = `Bearer ${token}`;
    }
  }
  return result;
}

/** Reads the {"error": "..."} body the API sends with every error status. */
async function errorMessage(response: Response): Promise<string> {
  try {
    const data = (await response.json()) as { error?: string };
    if (data.error) {
      return data.error;
    }
  } catch {
    // The body was not JSON, for example from a proxy in front of the API.
  }
  return `The server answered with status ${response.status}.`;
}
