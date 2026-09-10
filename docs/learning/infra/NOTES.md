# Teaching preferences and decisions

Original learning request: 2026-09-09. Runtime decision updated: 2026-09-10.

- Learn by deploying this repository's application, with small configuration,
  standard tools, high automation and a cost-efficient Hetzner VPS.
- The owner selected **Docker Compose first** after comparing host services
  with containers. Update deployment and learning documentation together;
  the earlier learning-only editing restriction is superseded by that request.
- Work on `master` for this session, as explicitly requested. Do not describe
  `deploy/` as living only in an implementation worktree.
- Stage 1 is single-server container operation and recovery; stage 2 is
  availability/scaling without Kubernetes; stage 3 is Kubernetes literacy.
- Explain images/containers, private networks, volumes, cgroups and failure
  recovery directly. Do not hide Linux fundamentals behind Compose commands.
- Use the pattern: explain, predict, run, inspect, deliberately fail, recover,
  record. A successful command is not evidence of every security property.
- Preserve the separate web/renderer boundary and one queue. Docker Compose
  does not imply a registry, Redis, a database or a cluster.
- Public hostname is `singularseed.art`; DNS ownership/access, staging hostname,
  budget, credentials and publication remain separate owner decisions.
