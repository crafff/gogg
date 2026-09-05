---
name: gogg-knowledge
description: Find, validate or propose durable GOGG engineering knowledge, investigate a stale/conflicting fact, or turn evidenced experience into a skill candidate. Excludes ordinary task status, raw player data and automatic model training.
---

# GOGG knowledge

Read `engineering/knowledge/README.md`. Start with the user's current scope and explicit
paths; use the knowledge tools for verified entries, then inspect original evidence before
acting. Do not infer that an archived business observation describes the new runtime.

Changes are small candidate patches with stable IDs, applicability and sources. Recheck
source identity and validity; mark outdated claims stale or superseded rather than silently
replacing their history. Conflicting evidence stays visible until resolved. Frequency,
confidence and multi-agent agreement are not independent proof.

Separate a fact correction from skill generalization. A proposed reusable skill needs a
reproducible failure, a limited scope, counterexamples and independent task evaluation using
`engineering/evals/protocols/learning.md`. Do not modify the evaluator or publish a candidate
while scoring it. These bootstrap skills are authored protocols, not demonstrated learning.

Never ingest `.local/`, credentials, hidden scoring data, raw logs or player profiles into
shared knowledge. A permitted source reference does not grant access to every linked object.
Run knowledge validation and a relevant query after accepted source changes. Search reads
authoritative facts directly; no disk-index rebuild is required. Review returned warnings.
