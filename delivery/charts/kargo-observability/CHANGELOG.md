# Changelog

All notable changes to `kargo-observability`. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versioning is semver.

## [Unreleased]

### Added

- `CustomResourceState` config for kube-state-metrics covering Promotion,
  Stage, Warehouse and Freight — nine metrics, no additional exporter.
- ClusterRole and binding granting the existing kube-state-metrics
  ServiceAccount read access to Kargo's CRDs, so no edit to that chart's own
  RBAC values is required.
- Eight alert rules, each individually toggleable with configurable `for` and
  `severity`, including `KargoMetricsMissing` — an `absent()` check without
  which every other rule fails open.
- A Grafana dashboard: current pipeline state, the two tables worth acting on,
  promotion and verification history, and freight age per warehouse.
- `values.schema.json` covering the full values contract.
