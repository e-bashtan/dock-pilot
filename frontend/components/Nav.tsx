"use client";

import { useEffect, useId, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { BrandLogo } from "@/components/BrandLogo";
import { LocaleSwitcher } from "@/components/LocaleSwitcher";
import { MobileQrModal } from "@/components/MobileQrModal";
import { useLogout } from "@/components/AuthGate";
import { useFleetMode } from "@/lib/fleet-mode";
import { useI18n } from "@/lib/i18n/context";

type NavItem = {
  href: string;
  labelKey:
    | "nav.sites"
    | "nav.databases"
    | "nav.backups"
    | "nav.payments"
    | "nav.notifications"
    | "nav.fleetServers"
    | "nav.fleetEvents"
    | "nav.fleetSettings";
  match: string;
};

const STANDALONE_LINKS: NavItem[] = [
  { href: "/sites", labelKey: "nav.sites", match: "sites" },
  { href: "/databases", labelKey: "nav.databases", match: "databases" },
  { href: "/backups", labelKey: "nav.backups", match: "backups" },
  { href: "/payments", labelKey: "nav.payments", match: "payments" },
  { href: "/notifications", labelKey: "nav.notifications", match: "notifications" },
  { href: "/fleet/settings", labelKey: "nav.fleetSettings", match: "fleet-settings" },
];

const MASTER_PRIMARY: NavItem[] = [
  { href: "/sites", labelKey: "nav.sites", match: "sites" },
  { href: "/fleet", labelKey: "nav.fleetServers", match: "fleet" },
  { href: "/payments", labelKey: "nav.payments", match: "payments" },
];

const MASTER_MORE: NavItem[] = [
  { href: "/fleet/events", labelKey: "nav.fleetEvents", match: "fleet-events" },
  { href: "/notifications", labelKey: "nav.notifications", match: "notifications" },
  { href: "/databases", labelKey: "nav.databases", match: "databases-only" },
  { href: "/backups", labelKey: "nav.backups", match: "backups-only" },
  { href: "/fleet/settings", labelKey: "nav.fleetSettings", match: "fleet-settings" },
];

function isPrimaryActive(pathname: string, match: string): boolean {
  if (match === "sites") {
    if (pathname === "/sites/new" || pathname.startsWith("/sites/new/")) {
      return false;
    }
    return pathname === "/sites" || pathname.startsWith("/sites/");
  }
  if (match === "databases-only") {
    return pathname === "/databases" || pathname.startsWith("/databases/");
  }
  if (match === "backups-only") {
    return pathname === "/backups" || pathname.startsWith("/backups/");
  }
  if (match === "fleet") {
    return pathname === "/fleet" || pathname.startsWith("/fleet/servers");
  }
  if (match === "fleet-events") {
    return pathname === "/fleet/events" || pathname.startsWith("/fleet/events/");
  }
  if (match === "fleet-settings") {
    return (
      pathname === "/fleet/settings" || pathname.startsWith("/fleet/settings/")
    );
  }
  return pathname === `/${match}` || pathname.startsWith(`/${match}/`);
}

export function Nav() {
  const logout = useLogout();
  const { t } = useI18n();
  const { isMaster } = useFleetMode();
  const pathname = usePathname() || "";
  const [qrOpen, setQrOpen] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [moreOpen, setMoreOpen] = useState(false);
  const moreRef = useRef<HTMLDivElement>(null);
  const moreMenuId = useId();

  useEffect(() => {
    setMenuOpen(false);
    setMoreOpen(false);
  }, [pathname]);

  useEffect(() => {
    if (!menuOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMenuOpen(false);
    };
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    window.addEventListener("keydown", onKey);
    return () => {
      document.body.style.overflow = prevOverflow;
      window.removeEventListener("keydown", onKey);
    };
  }, [menuOpen]);

  useEffect(() => {
    if (!moreOpen) return;
    const onDoc = (e: MouseEvent) => {
      if (!moreRef.current?.contains(e.target as Node)) setMoreOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMoreOpen(false);
    };
    document.addEventListener("mousedown", onDoc);
    window.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      window.removeEventListener("keydown", onKey);
    };
  }, [moreOpen]);

  const closeMenu = () => {
    setMenuOpen(false);
    setMoreOpen(false);
  };

  const primaryLinks = isMaster ? MASTER_PRIMARY : STANDALONE_LINKS;
  const mobileLinks = isMaster ? [...MASTER_PRIMARY, ...MASTER_MORE] : STANDALONE_LINKS;
  const moreActive =
    isMaster && MASTER_MORE.some((item) => isPrimaryActive(pathname, item.match));

  return (
    <>
      <nav className={`nav${menuOpen ? " nav-menu-open" : ""}`}>
        <div className="nav-bar">
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
            {t("nav.menu")}
          </button>
        </div>

        {menuOpen && (
          <button
            type="button"
            className="nav-backdrop"
            aria-label={t("nav.closeMenu")}
            onClick={closeMenu}
          />
        )}

        <div
          id="nav-menu"
          className={`nav-links${menuOpen ? " nav-links-open" : ""}`}
        >
          <div className="nav-drawer-head">
            <span className="nav-drawer-title">{t("nav.menu")}</span>
            <button
              type="button"
              className="btn btn-secondary nav-drawer-close"
              onClick={closeMenu}
            >
              {t("nav.closeMenu")}
            </button>
          </div>

          {/* Desktop: primary + More dropdown */}
          <div className="nav-primary nav-primary-desktop">
            {primaryLinks.map((item) => {
              const active = isPrimaryActive(pathname, item.match);
              return (
                <Link
                  key={`${item.href}-${item.match}`}
                  href={item.href}
                  className={`nav-link${active ? " nav-link-active" : ""}`}
                  aria-current={active ? "page" : undefined}
                  onClick={closeMenu}
                >
                  {t(item.labelKey)}
                </Link>
              );
            })}
            {isMaster && (
              <div className="nav-more" ref={moreRef}>
                <button
                  type="button"
                  className={`nav-link nav-more-btn${moreActive || moreOpen ? " nav-link-active" : ""}`}
                  aria-expanded={moreOpen}
                  aria-controls={moreMenuId}
                  onClick={() => setMoreOpen((v) => !v)}
                >
                  {t("nav.more")}
                  <span className="nav-more-caret" aria-hidden>
                    ▾
                  </span>
                </button>
                {moreOpen && (
                  <div id={moreMenuId} className="nav-more-menu" role="menu">
                    {MASTER_MORE.map((item) => {
                      const active = isPrimaryActive(pathname, item.match);
                      return (
                        <Link
                          key={`${item.href}-${item.match}`}
                          href={item.href}
                          role="menuitem"
                          className={`nav-more-item${active ? " nav-more-item-active" : ""}`}
                          aria-current={active ? "page" : undefined}
                          onClick={closeMenu}
                        >
                          {t(item.labelKey)}
                        </Link>
                      );
                    })}
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Mobile drawer: flat list, no nested More */}
          <div className="nav-primary nav-primary-mobile">
            {mobileLinks.map((item) => {
              const active = isPrimaryActive(pathname, item.match);
              return (
                <Link
                  key={`m-${item.href}-${item.match}`}
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
