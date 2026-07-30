export const LOCALES = ["en", "ru"] as const;
export type Locale = (typeof LOCALES)[number];
export const DEFAULT_LOCALE: Locale = "en";
export const LOCALE_COOKIE = "barn-locale";
const LEGACY_LOCALE_COOKIE = "dock-pilot-locale";

export function parseLocale(value?: string | null): Locale {
  return value === "ru" ? "ru" : "en";
}

/** Read locale cookie, accepting legacy dock-pilot-locale. */
export function readLocaleCookie(getCookie: (name: string) => string | undefined | null): Locale {
  return parseLocale(getCookie(LOCALE_COOKIE) ?? getCookie(LEGACY_LOCALE_COOKIE));
}
