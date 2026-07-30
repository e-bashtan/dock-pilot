const STORAGE_KEY = "barn-api-token";
const LEGACY_STORAGE_KEY = "dock-pilot-api-token";

function readStoredToken(storage: Storage, key: string): string | null {
  return storage.getItem(key);
}

export function getApiToken(): string | null {
  if (typeof window === "undefined") return null;

  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored) return stored;

  const legacyLocal = localStorage.getItem(LEGACY_STORAGE_KEY);
  if (legacyLocal) {
    localStorage.setItem(STORAGE_KEY, legacyLocal);
    localStorage.removeItem(LEGACY_STORAGE_KEY);
    return legacyLocal;
  }

  // One-time migration from older sessionStorage-only builds.
  const legacySession =
    readStoredToken(sessionStorage, STORAGE_KEY) ||
    readStoredToken(sessionStorage, LEGACY_STORAGE_KEY);
  if (legacySession) {
    localStorage.setItem(STORAGE_KEY, legacySession);
    sessionStorage.removeItem(STORAGE_KEY);
    sessionStorage.removeItem(LEGACY_STORAGE_KEY);
    return legacySession;
  }

  return null;
}

export function setApiToken(token: string): void {
  localStorage.setItem(STORAGE_KEY, token.trim());
  localStorage.removeItem(LEGACY_STORAGE_KEY);
  sessionStorage.removeItem(STORAGE_KEY);
  sessionStorage.removeItem(LEGACY_STORAGE_KEY);
}

export function clearApiToken(): void {
  localStorage.removeItem(STORAGE_KEY);
  localStorage.removeItem(LEGACY_STORAGE_KEY);
  sessionStorage.removeItem(STORAGE_KEY);
  sessionStorage.removeItem(LEGACY_STORAGE_KEY);
}

export const AUTH_LOGOUT_EVENT = "barn-auth-logout";
const LEGACY_AUTH_LOGOUT_EVENT = "dock-pilot-auth-logout";

export function notifyAuthLogout(): void {
  window.dispatchEvent(new Event(AUTH_LOGOUT_EVENT));
  window.dispatchEvent(new Event(LEGACY_AUTH_LOGOUT_EVENT));
}
