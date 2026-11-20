// In the browser the session lives in an HttpOnly cookie that JavaScript can
// neither read nor write, so there is no token to store (see backend session.go).
// This keeps a stolen-by-script token out of reach and keeps expo-secure-store,
// which has no web support, out of the web bundle.
export const usesCookieSession = true;

export async function getToken(): Promise<string | null> {
  return null;
}

export async function saveToken(_token: string): Promise<void> {}

export async function deleteToken(): Promise<void> {}
