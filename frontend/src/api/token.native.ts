import * as SecureStore from "expo-secure-store";

// On Android the login token is kept in the device's encrypted storage and sent
// as a bearer token. Expo picks this file over token.ts on native platforms.
const TOKEN_KEY = "session-token";

export const usesCookieSession = false;

export function getToken(): Promise<string | null> {
  return SecureStore.getItemAsync(TOKEN_KEY);
}

export function saveToken(token: string): Promise<void> {
  return SecureStore.setItemAsync(TOKEN_KEY, token);
}

export function deleteToken(): Promise<void> {
  return SecureStore.deleteItemAsync(TOKEN_KEY);
}
