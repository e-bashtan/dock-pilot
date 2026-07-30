"use client";

import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { BrandLogo } from "@/components/BrandLogo";
import { LocaleSwitcher } from "@/components/LocaleSwitcher";
import { exchangeQRCode } from "@/lib/api";
import { setApiToken } from "@/lib/auth-token";
import { resolveHomePath } from "@/lib/home-path";
import { useI18n } from "@/lib/i18n/context";

export default function MobileAuthClient() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { t } = useI18n();
  const [status, setStatus] = useState<"loading" | "error">("loading");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const code = searchParams.get("code")?.trim();
    if (!code) {
      setStatus("error");
      setError(t("mobileAuth.missingCode"));
      return;
    }

    let cancelled = false;

    (async () => {
      try {
        const token = await exchangeQRCode(code);
        if (cancelled) return;
        setApiToken(token);
        const home = await resolveHomePath();
        if (cancelled) return;
        router.replace(home);
      } catch (err) {
        if (cancelled) return;
        const message = err instanceof Error ? err.message : t("mobileAuth.failed");
        setStatus("error");
        setError(message);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [searchParams, router, t]);

  return (
    <div className="auth-screen">
      <div className="card auth-card mobile-auth-card">
        <LocaleSwitcher className="auth-locale" />
        <h1 className="auth-brand">
          <BrandLogo showVersion size="auth" />
        </h1>

        {status === "loading" && (
          <>
            <p className="mobile-auth-status">{t("mobileAuth.signingIn")}</p>
            <p className="auth-hint">{t("mobileAuth.pleaseWait")}</p>
          </>
        )}

        {status === "error" && (
          <>
            <div className="alert alert-error">{error}</div>
            <p className="auth-hint">{t("mobileAuth.retryHint")}</p>
          </>
        )}
      </div>
    </div>
  );
}
