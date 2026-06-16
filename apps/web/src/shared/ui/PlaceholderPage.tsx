import { useTranslation } from "react-i18next";

import { Tag } from "./Tag";

export interface PlaceholderPageProps {
  title: string;
  description: string;
  /** Optional badge text — defaults to the localized "Coming soon" tag. */
  badge?: string;
  /** Optional context line (e.g. the route param) rendered below the description. */
  context?: React.ReactNode;
}

/**
 * Generic "feature ships later" page body. Used by chunk 5's router
 * skeleton (champion-detail / summoner / login / me) so every route
 * has a visible, localized destination without committing to its
 * final shape.
 */
export function PlaceholderPage({
  title,
  description,
  badge,
  context,
}: PlaceholderPageProps) {
  const { t } = useTranslation("common");
  return (
    <section className="space-y-3" data-testid="placeholder-page">
      <div className="flex items-center gap-2">
        <h1 className="text-2xl font-semibold tracking-tight text-fg-default">
          {title}
        </h1>
        <Tag tone="accent">{badge ?? t("placeholder.comingSoon")}</Tag>
      </div>
      <p className="text-sm text-fg-muted">{description}</p>
      {context && <p className="text-xs text-fg-subtle">{context}</p>}
    </section>
  );
}
