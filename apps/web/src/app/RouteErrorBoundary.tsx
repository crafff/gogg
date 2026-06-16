import { useRouteError, isRouteErrorResponse } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { Button } from "@shared/ui";

// Route-level error boundary. React Router catches loader/render
// errors and routes them here via `errorElement`. The fallback is
// intentionally small — a real "something broke" screen with Sentry
// instrumentation arrives in Phase F.
export function RouteErrorBoundary() {
  const error = useRouteError();
  const { t } = useTranslation("common");

  const status = isRouteErrorResponse(error) ? error.status : null;
  const message = (() => {
    if (status === 404) return t("placeholder.notFound");
    if (isRouteErrorResponse(error)) return error.statusText || error.data;
    if (error instanceof Error) return error.message;
    return t("state.error");
  })();

  return (
    <section
      className="space-y-4 py-8 text-center"
      role="alert"
      data-testid="route-error"
    >
      <p className="text-3xl font-semibold text-fg-default">
        {status ?? t("state.error")}
      </p>
      <p className="text-sm text-fg-muted">{String(message)}</p>
      <Button variant="secondary" onClick={() => window.location.reload()}>
        {t("state.retry")}
      </Button>
    </section>
  );
}
