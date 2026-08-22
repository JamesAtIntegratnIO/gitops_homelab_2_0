# Contributing

This directory is a **self-contained package**. It is developed inside a
platform repository that consumes it, and it is expected to be extracted into
its own repository later:

```bash
git subtree split --prefix=delivery -b delivery-standalone
```

That only stays cheap if the package never grows a dependency on its host. Two
rules make that true, and `hack/extraction-test.sh` enforces both.

## Rule 1 — the one-way link rule

**Nothing under `delivery/` may reference a path outside `delivery/`.**

Documentation in the host repository links *into* this package. The package
never links out. A chart README that points at `../../docs/some-guide.md` is a
broken link the moment the package is extracted, and nobody notices until
after the split.

This covers markdown links, `helm` chart paths, `Dockerfile` `COPY` sources,
CI `uses:`/`include:` paths, and code that opens a file by relative path.

If you need to say something about the host repository, describe it in words
rather than linking to it. If the package genuinely needs a file that lives
outside it, that file belongs inside the package.

## Rule 2 — no environment assumptions

The package must run on somebody else's cluster, in somebody else's CI, against
somebody else's model. Concretely, nothing in `charts/` or `images/` may assume:

| Not assumed | How it is handled |
|---|---|
| A cluster name, domain, or namespace | A value. No `the-cluster`, no example.com. |
| A secret manager | Charts consume an **existing Secret by name**. ExternalSecret, Vault Agent, SOPS and friends belong to the consumer. |
| A CNI | Standard `NetworkPolicy` by default; `CiliumNetworkPolicy` with `toFQDNs` behind an opt-in flag. |
| A git host | `provider` is a value, and the agent goes through the `GitProvider` interface. |
| A model provider | `LLMProvider`, with no default. The values file must name one. |
| A CI system | The gate is a container with an exit code. CI adapters are thin and live in `ci/`. |
| A repository layout | The gate reads `.gitops-gate.yaml`. The image knows nothing about any particular repo. |

A grep for likely violations is part of the extraction test. It is not
exhaustive — the rule is the thing, the grep is a reminder.

## Every unit documents itself

Each chart and each image carries its own complete documentation, because after
extraction there is no other place for it to live:

- `README.md` — what it is, what it needs, how to install, what changed on upgrade
- `values.schema.json` (charts) — the machine-checkable values contract
- `CHANGELOG.md` — keep-a-changelog style
- `docs/` — reference material too long for the README

A change that alters behaviour and does not touch that unit's documentation is
incomplete.

## Design decisions

Anything load-bearing goes in [`adr/`](adr) as a short record: the context, the
decision, and what it costs. Write the ADR before the code when the decision is
contested, and note the alternatives you rejected — the next person needs to
know they *were* considered.

## Running the checks

```bash
hack/extraction-test.sh     # the two rules above, plus every chart templates
hack/lint.sh                # helm lint + values schema validation
```

## Versioning

Charts and images are versioned independently, semver, and published to a
container registry. The chart `appVersion` tracks the image it deploys.
