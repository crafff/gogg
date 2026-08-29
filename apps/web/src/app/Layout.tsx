import { Outlet, NavLink } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { LanguageSwitcher } from "@shared/i18n/LanguageSwitcher";
import { cn } from "@shared/lib/cn";
import { Skeleton } from "@shared/ui";
import { useSession } from "@features/auth";

// Global layout: brand strip + nav + LanguageSwitcher, then the
// routed page renders into <Outlet/>. Pages stay focused on their
// own content — no per-page header chrome.
export function Layout() {
  const { t } = useTranslation("common");
  const session = useSession();

  return (
    <div className="min-h-screen bg-surface text-fg">
      <header className="border-b border-border bg-surface-sunken/80 backdrop-blur">
        <div className="mx-auto flex max-w-5xl items-center justify-between gap-6 px-6 py-3">
          <NavLink
            to="/rankings"
            className="flex flex-col leading-none"
            aria-label={t("brand")}
          >
            <span className="text-xs uppercase tracking-wider text-fg-subtle">
              {t("brand")}
            </span>
            <span className="text-sm text-fg-muted">{t("tagline")}</span>
          </NavLink>

          <nav aria-label="primary" className="flex items-center gap-1 text-sm">
            <NavItem to="/rankings">{t("nav.rankings")}</NavItem>
            <NavItem to="/summoner">{t("nav.summoner")}</NavItem>
            <NavItem to="/tft">{t("nav.tft")}</NavItem>
          </nav>

          <div className="flex items-center gap-3">
            <LanguageSwitcher />
            {session.isPending ? (
              <Skeleton className="h-8 w-20" />
            ) : session.isError ? (
              <button
                type="button"
                className="rounded px-3 py-1.5 text-sm text-danger hover:bg-surface-overlay focus-visible:outline-none focus-visible:shadow-focus-ring"
                onClick={() => void session.refetch()}
              >
                {t("state.retry")}
              </button>
            ) : session.data?.me ? (
              <NavItem to="/me" variant="cta">
                {session.data.me.displayName}
              </NavItem>
            ) : (
              <NavItem to="/login" variant="cta">
                {t("nav.login")}
              </NavItem>
            )}
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-6 py-8">
        <Outlet />
      </main>
    </div>
  );
}

interface NavItemProps {
  to: string;
  children: React.ReactNode;
  variant?: "default" | "cta";
}

function NavItem({ to, children, variant = "default" }: NavItemProps) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        cn(
          "rounded px-3 py-1.5 transition",
          "focus-visible:outline-none focus-visible:shadow-focus-ring",
          variant === "cta"
            ? cn(
                "bg-accent text-fg-inverse hover:bg-accent-hover",
                isActive && "bg-accent-active",
              )
            : cn(
                "text-fg-muted hover:bg-surface-overlay hover:text-fg-default",
                isActive && "bg-surface-overlay text-accent",
              ),
        )
      }
    >
      {children}
    </NavLink>
  );
}
