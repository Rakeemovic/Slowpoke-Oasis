import { createContext, use, useEffect, useState, type ReactNode } from "react";

import { ApiError, setUnauthorizedHandler } from "@/api/client";
import { auth } from "@/api/endpoints";
import { deleteToken, getToken, saveToken, usesCookieSession } from "@/api/token";

export type AuthStatus = "loading" | "signedIn" | "signedOut";

interface AuthValue {
  status: AuthStatus;
  signIn: (username: string, password: string) => Promise<void>;
  signOut: () => Promise<void>;
}

const AuthContext = createContext<AuthValue | undefined>(undefined);

/** Keeps track of whether the user is logged in and offers login and logout. */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<AuthStatus>("loading");

  useEffect(() => {
    // Any 401 means the session is gone (expired, or ended on another device).
    setUnauthorizedHandler(() => {
      void deleteToken();
      setStatus("signedOut");
    });
    void checkSession().then(setStatus);
    return () => setUnauthorizedHandler(undefined);
  }, []);

  async function signIn(username: string, password: string) {
    const answer = await auth.login(username, password, usesCookieSession);
    if (answer?.token) {
      await saveToken(answer.token);
    }
    setStatus("signedIn");
  }

  async function signOut() {
    try {
      await auth.logout();
    } catch {
      // Logging out locally must work even if the server cannot be reached.
    }
    await deleteToken();
    setStatus("signedOut");
  }

  return <AuthContext value={{ status, signIn, signOut }}>{children}</AuthContext>;
}

export function useAuth(): AuthValue {
  const value = use(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return value;
}

/**
 * Finds out at app start whether an earlier login is still valid by asking
 * GET /me. The web app cannot look at its HttpOnly cookie, so it always has to ask.
 */
async function checkSession(): Promise<AuthStatus> {
  const token = await getToken();
  if (!usesCookieSession && !token) {
    return "signedOut";
  }
  try {
    await auth.me();
    return "signedIn";
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      return "signedOut";
    }
    // Server unreachable: a stored token is probably still fine, so the app
    // opens and each screen shows the connection error with a retry button.
    return token ? "signedIn" : "signedOut";
  }
}
