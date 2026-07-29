"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { BrandLogo } from "@/components/BrandLogo";
import { LocaleSwitcher } from "@/components/LocaleSwitcher";
import { MobileQrModal } from "@/components/MobileQrModal";
import { useLogout } from "@/components/AuthGate";
import { useFleetMode } from "@/lib/fleet-mode";
import { useI18n } from "@/lib/i18n/context";

const STANDALONE_LINKS = [
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

const MASTER_LINKS = [
  { href: "/fleet", labelKey: "nav.fleetServers" as const, match: "fleet" },
  { href: "/fleet/events", labelKey: "nav.fleetEvents" as const, match: "fleet-events" },
  { href: "/payments", labelKey: "nav.payments" as const, match: "payments" },
  {
    href: "/notifications",
    labelKey: "nav.notifications" as const,
    match: "notifications",
  },
  { href: "/sites", labelKey: "nav.fleetThisServer" as const, match: "sites" },
];

function isPrimaryActive(pathname: string, match: string): boolean {
  if (match === "sites") {
    if (pathname === "/sites/new" || pathname.startsWith("/sites/new/")) {
      return false;
    }
    return (
      pathname === "/sites" ||
      pathname.startsWith("/sites/") ||
      pathname === "/databases" ||
      pathname.startsWith("/databases/") ||
      pathname === "/backups" ||
      pathname.startsWith("/backups/")
    );
  }
  if (match === "fleet") {
    return pathname === "/fleet" || pathname.startsWith("/fleet/servers");
  }
  if (match === "fleet-events") {
    return pathname === "/fleet/events" || pathname.startsWith("/fleet/events/");
  }
  return pathname === `/${match}` || pathname.startsWith(`/${match}/`);
}

function isLocalSection(pathname: string): boolean {
  return (
    pathname === "/sites" ||
    pathname.startsWith("/sites/") ||
    pathname === "/databases" ||
    pathname.startsWith("/databases/") ||
    pathname === "/backups" ||
    pathname.startsWith("/backups/")
  );
}

export function Nav() {
  const logout = useLogout();
  const { t } = useI18n();
  const { isMaster, loading: fleetLoading } = useFleetMode();
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
  const links = isMaster ? MASTER_LINKS : STANDALONE_LINKS;
  const brandHref = isMaster ? "/fleet" : "/sites";
  const showContext = isMaster && !fleetLoading;
  const contextLabel = isLocalSection(pathname)
    ? t("nav.fleetContextLocal")
    : t("nav.fleetContextAll");

  return (
    <>
      <nav className="nav">
        <Link href={brandHref} className="nav-brand" onClick={closeMenu}>
          <BrandLogo showVersion showServerIP />
          {showContext && (
            <span className="nav-context muted">{contextLabel}</span>
          )}
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
            {links.map((item) => {
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
            {!isMaster && (
              <Link
                href="/sites/new"
                className="btn nav-new-site"
                onClick={closeMenu}
              >
                {t("nav.newSite")}
              </Link>
            )}
            {isMaster && (
              <Link
                href="/fleet/servers/new"
                className="btn nav-new-site"
                onClick={closeMenu}
              >
                {t("fleet.addServer")}
              </Link>
            )}
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
