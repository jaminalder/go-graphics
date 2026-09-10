# Compose is the chosen runtime; one queue is still the constraint

On 2026-09-10 the owner chose Docker Compose after comparing it with the existing
host-service design. Container packaging and operating experience are explicit
goals, sufficient reason to learn them before buying a VPS. The previous plan's
systemd-first preference was a tradeoff, not a product invariant.

What remains essential: a separate renderer failure boundary, one bounded web
queue, private communication, retained release identity and recoverable state.
Images package software; volumes and operator backups preserve external state;
Compose does not solve workspace loss or shared admission for multiple replicas.

The next exercise is the local three-container topology. Target-host OOM,
reboot, firewall, TLS, state locking and clean-host restoration still require
actual evidence. Choosing the runtime is not evidence that those drills passed.
