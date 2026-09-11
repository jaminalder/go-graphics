# C4 model and documentation conventions

## Scope and research basis

The architecture describes the application implemented in this repository: the local image generator and Singular Seed public studio. Provisioning tools are supporting operational software. The diagrams describe checked-in designs and configuration, not a live infrastructure inventory.

The [official C4 model](https://c4model.com/) organizes software architecture through system, container, component and code abstractions. A [context view](https://c4model.com/diagrams/system-context) introduces people and neighboring systems; it deliberately omits internal technology. A [container view](https://c4model.com/diagrams/container) then shows applications and data stores, their responsibilities, and communication.

Here, a C4 container means an executable application or data store, not necessarily a Docker container. The [official container abstraction](https://c4model.com/abstractions/container) explicitly treats predominantly server-rendered web applications as one container. Singular Seed's HTML templates, HTMX and small browser enhancement script therefore belong to the web application. The browser is an execution environment; it is not a separate single-page application. Source: [templates](../../internal/web/templates/page.html), [enhancement script](../../internal/web/assets/studio.js).

Each [component view](https://c4model.com/diagrams/component) decomposes exactly one container. Components group code behind a coherent interface. A package can supply multiple runtime roles: `renderjob.Manager` belongs to the web application; `Supervisor` belongs to the private supervisor; `Child` belongs to a disposable process. Shared packages compile into more than one application and do not imply a network dependency. A disposable child is represented separately because it has its own process lifecycle and pipe protocol.

The [code view](https://c4model.com/diagrams/code) is selective. Recipe identity and admission contracts merit a type diagram; exhaustive method inventories remain in source and `go doc`. C4 does not require all four levels everywhere. [Dynamic diagrams](https://c4model.com/diagrams/dynamic) explain important ordered interactions using elements from the static model. [Deployment diagrams](https://c4model.com/diagrams/deployment) map those applications onto the two environments supplied by the repository.

## Diagram notation

Diagrams use Mermaid, which is compatible with C4's [notation-independent approach](https://c4model.com/diagrams/notation). Each diagram has a title, explicit scope, typed elements, responsibilities, and a key. Boxes name the element type and implementation technology at container/component levels. An enclosing box is a stated system, container or deployment boundary. Solid arrows are directional relationships; labels name the action and, across processes, the protocol. Shapes and colors do not add undocumented meaning; size and position are layout only.

The [official review checklist](https://c4model.com/diagrams/checklist) guides review: each view must be understandable with its title and key, labels must match relationship direction, and technologies must be explicit at the appropriate level. Context omits deployment infrastructure; container views omit scaling and host configuration; component views do not turn subprocesses into in-process components.

## Source traceability and maintenance

Architecture pages cite implementation files rather than prior narratives. The [package map](../reference/packages.md), [sketch catalogue](../reference/sketches.md), API and configuration references cover the remaining code. Exact sketch flag help is generated from each current `Flags` implementation. Palette datasets remain unchanged because generators consume them.

Update the narrowest affected view when runtime responsibilities change. Changes to ports and process placement belong in deployment/configuration; changes to artist-facing controls belong in sketch or studio references. Keep the overview linked to those details. [Documentation tooling](../development/documentation.md) checks local destinations and headings, generates every page and copies linked source for portable offline reading.
