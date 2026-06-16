import { useTranslation } from "react-i18next";
import { useParams } from "react-router-dom";

import { PlaceholderPage } from "@shared/ui";

// Phase E wires this up to the on-demand EnrichSummonerWorkflow:
// the page will do a synchronous cache lookup, then fall back to
// triggering a Temporal workflow + SSE progress feed.
export function SummonerPage() {
  const { t } = useTranslation("common");
  const { region, name } = useParams<{ region: string; name: string }>();

  const context =
    region && name ? `${region.toUpperCase()} / ${name}` : undefined;

  return (
    <PlaceholderPage
      title={t("nav.summoner")}
      description={t("placeholder.summoner")}
      context={context}
    />
  );
}
