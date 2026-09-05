import { Navigate, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { Button, Skeleton } from "@shared/ui";

import { useSession } from "./useSession";

export function RequireSession({ children }: { children: React.ReactNode }) {
  const session = useSession();
  const location = useLocation();
  const { t } = useTranslation("common");

  if (session.isPending) {
    return (
      <div className="space-y-3" aria-label={t("auth.checkingSession")}>
        <Skeleton className="h-8 w-1/3" />
        <Skeleton className="h-24 w-full" />
      </div>
    );
  }
  if (session.isError) {
    return (
      <section className="space-y-3" role="alert">
        <p className="text-sm text-danger">{t("auth.sessionUnavailable")}</p>
        <Button variant="secondary" onClick={() => void session.refetch()}>
          {t("state.retry")}
        </Button>
      </section>
    );
  }
  if (!session.data.me) {
    const returnTo = `${location.pathname}${location.search}${location.hash}`;
    return (
      <Navigate
        to={`/login?${new URLSearchParams({ returnTo }).toString()}`}
        replace
      />
    );
  }
  return children;
}
