# Gitlab adapter

> **Unproven.** Written against documentation, not exercised against a live
> instance. Treat it as a starting point, and please report what was wrong.

See [`../README.md`](../README.md) for what an adapter has to do. The pieces
that differ per host:

- how to check out the pull request *and* its merge base
- the API call that reports a commit status
- where build artifacts are published for the agent to fetch
- whether pushes made with the CI token re-trigger the pipeline
