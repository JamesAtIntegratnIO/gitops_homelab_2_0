# Superseded by the `kargo-pipelines` chart

This chart is no longer rendered by anything. The `kargo-projects` addon points
at https://github.com/JamesAtIntegratnIO/bosun, which is the same chart generalised for
reuse plus promotion chains.

It is kept for one release as a rollback path, and because a chart that
vanishes in the same commit that repoints its consumer leaves nothing to
compare against if the switch turns out to be wrong.

The switch was verified byte-identical: rendered against the same values with
`nameLabel: kargo-projects`, all 111 objects matched exactly. The only
subsequent change was additive — five canary Stages, no renames, no deletions.

**Do not edit this copy.** Changes belong in
the `kargo-pipelines` chart, whose own `docs/` covers the target schema
and chaining. Delete this directory once the migration has held for a release.
