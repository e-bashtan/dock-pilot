"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useFleetMode } from "@/lib/fleet-mode";
import { useI18n } from "@/lib/i18n/context";

export function HomeRedirect() {
  const router = useRouter();
  const { loading, isMaster } = useFleetMode();
  const { t } = useI18n();

  useEffect(() => {
    if (loading) return;
    router.replace(isMaster ? "/fleet" : "/sites");
  }, [loading, isMaster, router]);

  return (
    <p className="muted" style={{ padding: "2rem 1rem", textAlign: "center" }}>
      {t("common.loading")}
    </p>
  );
}
