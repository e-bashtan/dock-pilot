"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { BrandLogo } from "@/components/BrandLogo";
import { LocaleSwitcher } from "@/components/LocaleSwitcher";
import { MobileQrModal } from "@/components/MobileQrModal";
import { useLogout } from "@/components/AuthGate";
import { useI18n } from "@/lib/i18n/context";

export function Nav() {
  const logout = useLogout();
  const { t } = useI18n();
  const [qrOpen, setQrOpen] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    if (!menuOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMenuOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [menuOpen]);

  const closeMenu = () => setMenuOpen(false);

  return (
    <>
      <nav className="nav">
        <Link href="/sites" className="nav-brand" onClick={closeMenu}>
          <BrandLogo showVersion showServerIP />
        </Link>
        <button
          type="button"
          className="nav-toggle btn btn-secondary"
          aria-expanded={menuOpen}
          aria-controls="nav-menu"
          onClick={() => setMenuOpen((open) => !open)}
        >
          {menuOpen ? t("nav.closeMenu") : t("nav.menu")}
        </button>
        <div
          id="nav-menu"
          className={`nav-links${menuOpen ? " nav-links-open" : ""}`}
        >
          <Link href="/sites" onClick={closeMenu}>
            {t("nav.sites")}
          </Link>
          <Link href="/databases" onClick={closeMenu}>
            {t("nav.databases")}
          </Link>
          <Link href="/backups" onClick={closeMenu}>
            {t("nav.backups")}
          </Link>
          <Link href="/payments" onClick={closeMenu}>
            {t("nav.payments")}
          </Link>
          <Link href="/notifications" onClick={closeMenu}>
            {t("nav.notifications")}
          </Link>
          <Link href="/sites/new" onClick={closeMenu}>
            {t("nav.newSite")}
          </Link>
          <button
            type="button"
            className="btn btn-secondary nav-mobile-qr"
            onClick={() => {
              closeMenu();
              setQrOpen(true);
            }}
          >
            {t("nav.mobile")}
          </button>
          <LocaleSwitcher />
          <button
            type="button"
            className="btn btn-secondary nav-logout"
            onClick={() => {
              closeMenu();
              logout();
            }}
          >
            {t("nav.logout")}
          </button>
        </div>
      </nav>
      <MobileQrModal open={qrOpen} onClose={() => setQrOpen(false)} />
    </>
  );
}
