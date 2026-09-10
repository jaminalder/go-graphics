# Remaining infrastructure work

Updated for the owner-approved Compose runtime, 2026-09-10. This supersedes the
old list that treated containers as optional homework. See the deployment
runbook for executable instructions, and launch-gates for proof still required.

## Implemented direction

Three containers, pinned base images, static Go runtime, explicit proxy trust,
separate resource limits/users, private Unix socket, mounted cache/TLS state,
bounded logs, image archives and Compose smoke/drain/activation/rollback.
Host application units are removed. Terraform environment is configurable;
cloud-init remains immutable. A host bootstrap script installs pinned Docker
packages. Automatic OS reboot is disabled. Staging approval is distinct from
public-launch approval.

## Stage 1 gaps to close through operation

1. Choose/bootstrap independent state storage; prove two-client locking,
   interrupted-operation recovery and restoration with actual backend semantics.
2. Confirm total budget, DNS ownership, staging hostname, SSH CIDRs, operator
   machine and recovery objective. The public name/domain choice is already made.
3. Install and verify on Ubuntu amd64. Apply Docker-aware forwarding policy,
   confirm console access, test IPv4/IPv6 refusals and reboot persistence.
4. Prove live TLS issuance/renewal, mounted certificate persistence and a staged
   deploy/rollback. Host bootstrap is not verified by a Compose laptop rehearsal.
5. Measure real target artwork capacity and renderer OOM isolation under load;
   inspect cgroup events even if the supervisor remains running.
6. Rehearse independent volume/archive backup and second-machine restoration.
7. Define a disk reserve and explicit release/image retention policy. Keep the
   previous release and any renderer needed for retained editions; no global prune.
8. Choose external uptime monitoring, alert recipient, escalation hours, and
   scheduled update/reboot ownership. Add graphs only after an operational need.
9. Tighten bootstrap operator sudo access after the initial host is working;
   access to Docker is effectively host administration, not an app permission.
10. Verify slow downloads against proxy/app write timeouts and enable HSTS only
    after HTTPS is demonstrated.

A registry and automated image publishing are optional later distribution work;
checksummed `docker save` archives avoid making a registry a first-stage dependency.
CI must not gain production credentials accidentally. Stage 2 may address rolling
releases or additional renderers only after measured need and a single-admission
story. Stage 3 Kubernetes remains a separate lab.
