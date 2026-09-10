# Infrastructure learning glossary

**Image:** immutable packaged filesystem and execution metadata. An image does
not contain a running process, live volume data or the host kernel.

**Container:** a process environment created from an image with configured
namespaces, resource limits, mounts and networking. It shares the Linux kernel.

**Tag / digest / image ID:** a tag is a movable name; a digest identifies image
content; Docker's local image ID identifies its configuration. Our release
archive records exact image IDs and its own checksum, not only tags.

**Compose service / project:** a service declares a container role. A project
groups its services, networks and volumes. The fixed production project owns
one web queue; another project would create another queue.

**Namespace:** Linux's isolation of a process view, such as networking or PIDs.
**Cgroup:** accounting and limits for a group of processes, including children.
These are different mechanisms, both used by containers.

**Published port:** forwarding from a host address/port into a container.
An internal listening port is not automatically a public published port.
Docker forwarding can follow a different firewall path from host INPUT.

**Loopback:** the current network namespace's own interface. A container's
`127.0.0.1` is not another container or the VPS host.

**Volume / bind mount:** storage mounted outside the image's writable layer.
A named volume is managed by Docker; a bind mount exposes an explicit host path.
Neither is automatically backed up. Our socket/cache/TLS mounts are named volumes.

**Infrastructure as code:** declarations of cloud objects and their lifecycle,
not a synonym for all host and application configuration.

**Bootstrap:** host first-boot facts and runtime prerequisites. Separate from
an application release; changing cloud-init may mean replacing the host.

**Release:** a source revision's retained image archive, configuration, image
IDs, edition manifest and checksums. Distinct from an artwork edition.

**Admission:** accepting expensive render work into the bounded application
queue. Separate from HTTP request limits and packet-level filtering.

**Edge:** the public TLS endpoint, here the Caddy container. It supplies
sanitized client identity to web over the private proxy network.

**Isolation:** a renderer failure fails a job while browsing survives. It must
be demonstrated under the configured limits, not inferred from container names.

**Liveness / readiness:** liveness answers whether web can serve; renderer
readiness answers whether compatible generation is available. An unhealthy
Docker container does not automatically restart just because it is unhealthy.

**Drain:** stop admission and wait for queued/running work before replacement.
It does not preserve in-memory workspaces or guarantee zero downtime.

**Vertical / horizontal scale:** a larger machine / additional processes or
machines. Additional web processes require a plan for shared admission/state.

**State:** Terraform resource mapping, operator configuration, release archives
and TLS data need recovery copies. In-memory workspaces, cache and sockets are
disposable by this product's contract.
