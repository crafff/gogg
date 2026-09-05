import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { useLogout, useSession } from "@features/auth";
import { Button, Tag } from "@shared/ui";

export function MePage() {
  const { t } = useTranslation("common");
  const navigate = useNavigate();
  const session = useSession();
  const logout = useLogout();
  const account = session.data?.me;

  if (!account) return null;

  async function signOut() {
    try {
      await logout.mutateAsync();
      navigate("/rankings", { replace: true });
    } catch {
      // Keep the page and expose the error below when revocation fails.
    }
  }

  return (
    <section className="space-y-6">
      <header>
        <p className="text-xs font-medium uppercase tracking-wider text-accent">
          {t("auth.account")}
        </p>
        <h1 className="mt-1 text-3xl font-semibold text-fg-default">
          {account.displayName}
        </h1>
        {account.email && (
          <p className="mt-1 text-sm text-fg-muted">{account.email}</p>
        )}
      </header>

      <div className="rounded-xl border border-border bg-surface-raised p-5 shadow-card">
        <div className="flex items-center gap-4">
          <div
            className="flex h-14 w-14 items-center justify-center rounded-full bg-accent/15 text-xl font-semibold text-accent"
            aria-hidden="true"
          >
            {account.displayName.slice(0, 1).toUpperCase()}
          </div>
          <div>
            <p className="font-medium text-fg-default">
              {t("auth.connectedAccounts")}
            </p>
            <div className="mt-2 flex flex-wrap gap-2">
              {account.identities.map((identity) => (
                <Tag key={identity.provider} tone="accent">
                  {identity.provider === "google"
                    ? "Google"
                    : identity.provider}
                </Tag>
              ))}
            </div>
          </div>
        </div>
      </div>

      <div className="space-y-2">
        <Button
          variant="secondary"
          onClick={() => void signOut()}
          disabled={logout.isPending}
        >
          {logout.isPending ? t("auth.signingOut") : t("auth.signOut")}
        </Button>
        {logout.isError && (
          <p className="text-sm text-danger" role="alert">
            {t("auth.signOutFailed")}
          </p>
        )}
      </div>
    </section>
  );
}
