import { Navigate, useSearchParams } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { useAuthProvidersQuery } from "@shared/api";
import { Button, Skeleton } from "@shared/ui";

import { safeReturnTo } from "./redirects";
import { useSession } from "./useSession";

export function LoginPage() {
  const { t } = useTranslation("common");
  const [searchParams] = useSearchParams();
  const session = useSession();
  const providers = useAuthProvidersQuery(undefined, {
    retry: 1,
    enabled: session.isSuccess && !session.data.me,
  });
  const returnTo = safeReturnTo(searchParams.get("returnTo"));
  const errorCode = searchParams.get("error");

  if (session.isPending) {
    return <Skeleton className="mx-auto h-64 max-w-md rounded-xl" />;
  }
  if (session.isError) {
    return (
      <section
        className="mx-auto max-w-md space-y-3 rounded-xl border border-border bg-surface-raised p-6 shadow-card"
        role="alert"
      >
        <p className="text-sm text-danger">{t("auth.sessionUnavailable")}</p>
        <Button variant="secondary" onClick={() => void session.refetch()}>
          {t("state.retry")}
        </Button>
      </section>
    );
  }
  if (session.data?.me) {
    return <Navigate to={returnTo} replace />;
  }

  const googleEnabled = providers.data?.authProviders.some(
    (provider) => provider.id === "google",
  );

  return (
    <section className="mx-auto max-w-md rounded-xl border border-border bg-surface-raised p-6 shadow-card sm:p-8">
      <p className="text-xs font-medium uppercase tracking-wider text-accent">
        {t("brand")}
      </p>
      <h1 className="mt-2 text-2xl font-semibold text-fg-default">
        {t("auth.title")}
      </h1>
      <p className="mt-2 text-sm leading-6 text-fg-muted">
        {t("auth.description")}
      </p>

      {errorCode && (
        <p
          className="mt-4 rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger"
          role="alert"
        >
          {t(errorMessageKey(errorCode))}
        </p>
      )}

      <div className="mt-6">
        {providers.isPending ? (
          <Skeleton className="h-11 w-full" />
        ) : providers.isError ? (
          <div className="space-y-3" role="alert">
            <p className="text-sm text-danger">
              {t("auth.providersUnavailable")}
            </p>
            <Button
              variant="secondary"
              onClick={() => void providers.refetch()}
            >
              {t("state.retry")}
            </Button>
          </div>
        ) : googleEnabled ? (
          <Button asChild size="lg" className="w-full">
            <a
              href={`/oauth/start/google?${new URLSearchParams({ returnTo }).toString()}`}
            >
              {t("auth.continueWithGoogle")}
            </a>
          </Button>
        ) : (
          <p className="rounded-lg bg-surface-overlay p-3 text-sm text-fg-muted">
            {t("auth.googleNotConfigured")}
          </p>
        )}
      </div>
      <p className="mt-5 text-xs leading-5 text-fg-subtle">
        {t("auth.privacyNote")}
      </p>
    </section>
  );
}

function errorMessageKey(
  code: string,
): "auth.errorCancelled" | "auth.errorExpired" | "auth.errorGeneric" {
  switch (code) {
    case "cancelled":
      return "auth.errorCancelled";
    case "expired":
      return "auth.errorExpired";
    default:
      return "auth.errorGeneric";
  }
}
