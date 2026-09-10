# Questions to answer in writing

Answer in this file, under each question, as you learn. Explain each tool against the failure it addresses and record evidence
separately from the decision to use it. Copy a question into a
[learning record](learning-records/README.md) only when the answer changed
your mind.

Status key: `unanswered` · `partially decided` · `decided` · `revisit after stage N`

---

## Before stage 1 (product and ownership)

These overlap the owner decisions in [`docs/web/README.md`](../../web/README.md).
Infrastructure cannot invent them.

1. **What is the public hostname, and who owns the DNS?**
   Status: partially decided
   Public hostname: `singularseed.art` (owner, 2026-09-10). DNS ownership/access
   and the staging hostname remain unanswered.
   Why it matters: Caddy automatic HTTPS and `ART_ORIGIN` are meaningless
   without one canonical name. Terraform can manage records only after this.

2. **What is the actual monthly ceiling, including VAT, IPv4, backups, domain,
   state storage, and monitoring?**
   Status: unanswered
   Why it matters: CX23 + IPv4 + backups is already a known subtotal in
   research.md. Object storage for Terraform state can dominate if you pick a
   product with a high minimum.

3. **Which machine is allowed to run Terraform against production?**
   Status: unanswered
   Why it matters: remote state is how a second laptop does not fork the
   infrastructure. “My laptop and also CI” is two identities, two secrets.

4. **What is the restore time you will tell yourself in public?**
   Status: unanswered
   Why it matters: the plan proposes two hours and no high-availability claim.
   If you need five nines, you are not on one VPS, and stage 2 will not save
   you.

5. **Terraform or OpenTofu?**
   Status: unanswered
   Why it matters: the implementation pins Terraform 1.14.9. OpenTofu is the
   open-source answer if HashiCorp's licence bothers you. Switching is a
   decision, not a drive-by.

---

## Stage 1 — single server

6. **Where does Terraform state live, and did locking actually work from two
   clients?**
   Status: unanswered
   Why it matters: Hetzner Object Storage might be fine; “S3 compatible” is
   not a proof. Write the experiment: two `apply`s, one waits, pull an old
   version, restore it.

7. **If cloud-init changes, do you replace the server or ignore `user_data`
   after first boot?**
   Status: unanswered
   Why it matters: `user_data` is on the `hcloud_server` resource. A casual
   edit can be a destroy/recreate. Immutable bootstrap vs mutable bootstrap is
   the first IaC design fork.

8. **Who may SSH, from which CIDRs, over IPv4 and IPv6, and how do you recover
   when your home IP changes?**
   Status: unanswered
   Why it matters: the firewall is the real SSH policy. Console recovery is
   the escape hatch. WireGuard later is an answer, not a default.

9. **Which secrets are allowed on the VPS, and which must never touch it?**
   Status: unanswered
   Why it matters: the renderer should have no cloud credentials. Terraform
   tokens should not live on the app host. Caddy's ACME account *does* live
   there and must be backed up.

10. **What exactly do you mean by rate limit?**
    Status: unanswered
    Why it matters: collapsing edge, admission, and anti-abuse into one Caddy
    module is how people end up with a custom Caddy build. Write three
    numbers: max body, batches/minute, and what happens to the rest of the
    internet scanners.

11. **Why are we using containers, and which boundaries must they preserve?**
    Status: decided
    Owner selected Docker Compose on 2026-09-10 for standard image packaging,
    reproducible local topology and container operations learning. Keep Caddy,
    web and renderer separate, one queue, explicit limits, private Unix socket,
    no renderer networking or Docker control socket. See ADR 0004.
    Exercise still required: explain images, volumes, namespaces and cgroups
    against the running configuration; prove failure isolation on the target.

12. **When do security updates reboot the box, and who is awake?**
    Status: partially decided
    Automatic reboot is disabled in cloud-init. The maintenance window and
    responsible operator remain unanswered.
    Why it matters: Automatic reboot of a single VPS *is* downtime. That is an
    SLO choice.

13. **Which image archives, operator files and volumes have independent backups?**
    Status: unanswered
    Why it matters: those slots die with the server. List: state, releases,
    `/etc/art`, Caddy data/config volumes, credentials. The image cache is not on the list
    unless you changed the product promise.

14. **What is the one alert that pages you, and what do you do in ten minutes?**
    Status: unanswered
    Why it matters: a metrics stack without a recipient is decoration.
    Gallery down, disk < reserve, renderer restart loop, cert expiry are the
    candidates; pick one first.

15. **How does a release get onto the box without Terraform knowing?**
    Status: unanswered
    Why it matters: scp from your laptop, pull from GitHub Releases, or CI
    with a deploy key are three different trust stories. Stage 1 can stay
    “laptop uploads”; write that down so stage 2 does not silently add CI
    production credentials.

---

## Stage 2 — downtime and scale

16. **How many in-flight explorations may a deploy destroy?**
    Status: unanswered
    Why it matters: if the answer is “all of them, they can retry”, drain-
    and-restart is correct and you should not build a second queue. If the
    answer is “none”, you need persistence or a true handoff.

17. **Where does the single admission token live if two `artweb` processes
    exist?**
    Status: unanswered
    Why it matters: this is the question Kubernetes cannot answer for you.
    File lock, localhost Redis, SQLite, or “there are never two”. Pick one
    before Caddy load-balances.

18. **Is the bottleneck CPU, RAM, disk, or queue wait?**
    Status: unanswered
    Why it matters: vertical scale, a dedicated renderer, a larger cache
    disk, and a second replica are four different purchases. Measure on the
    CX23 before changing topology.

19. **What is the private network between web and renderer if they split?**
    Status: unanswered
    Why it matters: Hetzner private networks vs WireGuard vs public bind +
    firewall. The current renderer uses `network_mode: none` and a Unix socket.
    Splitting hosts is an application-protocol change, not only a Compose-file
    change.

20. **What would make you add a database?**
    Status: unanswered
    Why it matters: durable public links, shared workspaces, or zero-downtime
    sessions. If none of those are promised, SQLite is a solution looking for
    a problem.

21. **What is “autoscaling” as a sentence with numbers?**
    Status: unanswered
    Example worth stealing: “If queue wait p95 > 20s for 10 minutes, alert me;
    I may resize to CX33. No machine creates itself.” Until you can write
    that sentence, do not script `hcloud server create`.

22. **Would a second *region* buy anything this product needs?**
    Status: unanswered
    Why it matters: latency for a 15-second render is dominated by CPU, not
    kilometres. Multi-region is hyperscaler muscle memory.

---

## Stage 3 — Kubernetes

23. **Which stage-2 failure is still open, in one sentence?**
    Status: unanswered
    Why it matters: if the sentence is empty, k3s is a lab. If it is “two
    people ship twice a day and Compose definitions drift”, you have an operators
    problem. If it is “I need a rolling update of 12 services”, you have a
    different product.

24. **What idle RAM did k3s cost on an empty node?**
    Status: unanswered
    Why it matters: that RAM was renderer budget on CX23. Write the number
    before talking about “lightweight Kubernetes”.

25. **Did a rolling Deployment keep a single global admission?**
    Status: unanswered
    Why it matters: this is the exam. If you had to disable generation during
    the roll, Kubernetes did not give you the property you named in question
    16.

26. **File count: `deploy/` versus the k3s manifests plus Terraform that
    still creates the VPS.**
    Status: unanswered
    Why it matters: Kubernetes does not delete Terraform. It adds a second
    desired-state language. Small config is the sum.

27. **Are you learning Kubernetes, or operating this studio with Kubernetes?**
    Status: unanswered
    Why it matters: both are valid; they have different success criteria. A
    lab cluster may be destroyed. Production must still restore in your
    stated two hours.

---

## Recurring questions (ask every time a tool appears)

28. **Which layer does this tool own, and which file does it delete?**
    If it owns two layers, reject it. If it adds files and deletes none, it
    is probably a parallel universe.

29. **What is the failure you can demonstrate without it, and with it?**
    If you cannot demo the difference, you are collecting tools.

30. **Can you restore it from a second machine with the documents in Git plus
    one secret store?**
    If the answer depends on a laptop directory named `~/stuff`, it is not
    infrastructure as code yet.

## Container-first stage-1 exercises

31. Which files are in an image, a writable layer, a named volume and the host?
32. Which service can reach which ports? Why is the web admin port still private?
33. Why does a container's unhealthy status not automatically restart it?
34. Which exact image IDs run now, and how do you restore the previous release?
35. Where do Docker-published packets meet firewall policy over IPv4 and IPv6?
36. What does Compose recreation preserve, and what does it lose?
