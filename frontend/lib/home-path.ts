import { api } from "@/lib/api";

/** Home route after login: Master → /fleet, otherwise → /sites. */
export async function resolveHomePath(): Promise<"/fleet" | "/sites"> {
  try {
    const settings = await api.getFleetSettings();
    if (settings.mode === "master") {
      return "/fleet";
    }
  } catch {
    /* fall through */
  }
  return "/sites";
}
