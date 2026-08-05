"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useServersMode } from "@/lib/servers-mode";
import { resolveHomePath } from "@/lib/home-path";
import { useI18n } from "@/lib/i18n/context";

export function HomeRedirect() {
  const router = useRouter();
  const { loading } = useServersMode();
  const { t } = useI18n();

  useEffect(() => {
    if (loading) return;
    let cancelled = false;
    (async () => {
      const home = await resolveHomePath();
      if (!cancelled) {
        router.replace(home);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [loading, router]);

  return (
    <p className="muted" style={{ padding: "2rem 1rem", textAlign: "center" }}>
      {t("common.loading")}
    </p>
  );
}
