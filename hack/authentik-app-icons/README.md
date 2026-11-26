# authentik application icons

`icons/<slug>.svg|png` is the icon for the authentik application with that
slug. `build.sh` turns the folder into the `authentik-app-icons` ConfigMap,
mounted into authentik-server at `/web/dist/assets/images/apps`. Each
application's blueprint sets `meta_icon: /static/dist/assets/images/apps/<file>`
(authentik's static file backend serves any `/static/...` path as-is).

Adding an app: drop its icon here, run `./build.sh`, set `meta_icon` in its
blueprint entry. Prefer SVG, and a light variant: the authentik theme is dark.

Sources (2026-11-26): most from homarr-labs/dashboard-icons (`argo-cd`,
`grafana`, `open-webui-light`, `qdrant`, `umami-light`, `glance` for homepage,
`sonarr`, `radarr`, `bazarr`, `sabnzbd`, `mealie`, `hermes-agent-alt-light`);
kargo from akuity/kargo `ui/public/kargo-logo-head-only.png`; bosun from
IntegratnIO/bosun `site/public/favicon.svg`; specmarshal uses the Integratn mark
(`hack/authentik-theme`). Logos are their projects' trademarks.
