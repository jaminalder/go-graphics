# Teaching preferences and decisions

Original learning request: 2026-09-09. Runtime decision updated: 2026-09-10.

- Learn by deploying this repository's application, with small configuration,
  standard tools, high automation and a cost-efficient Hetzner VPS.
- The owner selected **Docker Compose first** after comparing host services
  with containers. That implementation session updated deployment and learning
  docs together. The visual-learning revision below is confined to this folder.
- Historical Compose implementation session: the owner requested work on
  `master`. This is not standing authorization to bypass the repository’s
  current worktree workflow. `deploy/` is implemented on `master`.
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

## Visual learning revision — 2026-09-10

- The owner knows Docker, Linux and CI/CD basics. Focus on how this system fits
  together: responsibility boundaries, request flow, release identity and failure.
- Keep all learning work within `docs/learning/infra/`, separate from application,
  deployment and other project documentation. Link to those sources; do not move
  or duplicate their implementations.
- Use browser-readable HTML, real inline SVG diagrams and reusable assets. The
  atlas is `index.html`; short lessons live in `lessons/`, compressed reference
  in `reference/`, shared presentation/interaction code in `assets/`.
- Use prediction, immediate feedback, recall and spaced return. Browser notes
  are optional self-assessments, never automatic evidence of mastery. Do not add
  a learning record just because a lesson was authored or opened.
- The mission is unchanged: operate and recover the chosen one-host Compose
  studio before considering scaling or a Kubernetes lab.
