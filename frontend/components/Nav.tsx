"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { BrandLogo } from "@/components/BrandLogo";
import { LocaleSwitcher } from "@/components/LocaleSwitcher";
import { MobileQrModal } from "@/components/MobileQrModal";
import { useLogout } from "@/components/AuthGate";
import { useI18n } from "@/lib/i18n/context";

const PRIMARY_LINKS = [
  { href: "/sites", labelKey: "nav.sites" as const, match: "sites" },
  { href: "/databases", labelKey: "nav.databases" as const, match: "databases" },
  { href: "/backups", labelKey: "nav.backups" as const, match: "backups" },
  { href: "/payments", labelKey: "nav.payments" as const, match: "payments" },
  {
    href: "/notifications",
    labelKey: "nav.notifications" as const,
    match: "notifications",
  },
];

function isPrimaryActive(pathname: string, match: string): boolean {
  if (match === "sites") {
    if (pathname === "/sites/new" || pathname.startsWith("/sites/new/")) {
      return false;
    }
    return pathname === "/sites" || pathname.startsWith("/sites/");
  }
  return pathname === `/${match}` || pathname.startsWith(`/${match}/`);
}

export function Nav() {
  const logout = useLogout();
  const { t } = useI18n();
  const pathname = usePathname() || "";
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
          <div className="nav-primary">
            {PRIMARY_LINKS.map((item) => {
              const active = isPrimaryActive(pathname, item.match);
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={`nav-link${active ? " nav-link-active" : ""}`}
                  aria-current={active ? "page" : undefined}
                  onClick={closeMenu}
                >
                  {t(item.labelKey)}
                </Link>
              );
            })}
          </div>
          <div className="nav-actions">
            <Link
              href="/sites/new"
              className="btn nav-new-site"
              onClick={closeMenu}
            >
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
        </div>
      </nav>
      <MobileQrModal open={qrOpen} onClose={() => setQrOpen(false)} />
    </>
  );
}
