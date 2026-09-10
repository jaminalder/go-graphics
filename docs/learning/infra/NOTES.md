# Preferences for later sessions

Recorded 2026-09-09 from the request that created this track.

- Keep existing web/infra plans untouched; add files under `docs/learning/`
  only.
- The argument to test is: a robust public runtime can live on a plain cloud
  VPS; AWS + Kubernetes is not a prerequisite.
- Desired shape of learning: stage 1 single server, stage 2 zero-downtime and
  scaling without Kubernetes, stage 3 Kubernetes only if stage 2 runs out of
  road.
- Values: small config, high automation, standard open-source tools,
  cost-efficient Hetzner.
- “Containerization” and “proxy rate limits” were named as stage-1 wants.
  Treat them as questions to answer against the existing systemd/Caddy/Go
  design, not as missing defaults.
- Teaching should stay tied to this repository's `deploy/` artifacts and
  `docs/web/security-and-operations.md`, not a generic VPS tutorial.
