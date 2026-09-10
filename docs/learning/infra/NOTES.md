# Documentation preferences and decisions

Updated 2026-09-10 after the owner reviewed and rejected the teaching-course format.

- Provide direct system and operational information. Do not turn this material
  into lessons, quizzes, exercises, spaced reviews, progress tracking or a course.
- The owner knows Docker, Linux and CI/CD basics. Explain this system's specific
  connections, configuration files, responsibilities and reasons for separation.
- The requested deliverable is one HTML reference with useful diagrams. Keep all
  documentation changes inside `docs/learning/infra/`; link to application and
  deployment sources instead of changing or duplicating their implementations.
- `index.html` replaces the rejected HTML course. The previous course pages,
  shared course assets and browser-note features were removed. Earlier Markdown
  planning files are background, not instructions to resume teaching.
- The selected runtime remains Docker Compose on one Hetzner VPS, with Caddy,
  separate web/renderer containers, one queue and no application database.
- Public hostname: `singularseed.art`. Deployment/provisioning requirements and
  operational proof belong in the existing runbook and launch-gate documents.
- Follow the repository's worktree and integration rules. The earlier request
  to implement Compose on master was specific to that historical session.
