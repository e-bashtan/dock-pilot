/**
 * Try to parse multi-database error like "pdb_admin: ...; barn: ..."
 * Returns array of { db, error } or null if not parseable.
 */
export function parseBackupError(
  message: string,
): Array<{ db: string; error: string }> | null {
  if (!message) return null;
  const parts = message.split(";").map((p) => p.trim()).filter(Boolean);
  if (parts.length === 0) return null;

  const parsed: Array<{ db: string; error: string }> = [];
  for (const part of parts) {
    const colonIdx = part.indexOf(":");
    if (colonIdx > 0 && colonIdx < part.length - 1) {
      const db = part.slice(0, colonIdx).trim();
      const error = part.slice(colonIdx + 1).trim();
      if (db && error) {
        parsed.push({ db, error });
      }
    }
  }

  if (parsed.length > 0 && parsed.length === parts.length) {
    return parsed;
  }
  return null;
}
