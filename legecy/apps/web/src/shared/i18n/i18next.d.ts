import "i18next";

import type { resources } from "./resources";

// Module augmentation so `useTranslation('rankings')` gets autocomplete
// for keys, and the default namespace ('common') is type-checked when
// no namespace is passed.
declare module "i18next" {
  interface CustomTypeOptions {
    defaultNS: "common";
    resources: (typeof resources)["zh-CN"];
  }
}
