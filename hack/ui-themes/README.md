# ArgoCD and Kargo in the IntegratnIO brand

The ArgoCD and Kargo UIs wear the same design system as the landing page
(`addons/cluster-roles/control-plane/addons/homepage/glance/glance.css`) and
integratn.io: graphite ground, flat hairline panels, colour only on a verdict
(healthy/synced green, out of sync/missing amber, degraded/failed red; progressing,
suspended and unknown stay neutral), system sans and mono, 6px radius.

Neither tool takes a theme, only CSS, so the stylesheets here re-colour each
tool's own rules.

## How it is served

| Piece | Where |
|---|---|
| Both stylesheets | ConfigMap `argocd-styles-cm` (`clusters/the-cluster/addons/argo-cd/manifests/argocd-styles-cm.yaml`), **generated** by `build.sh` |
| ArgoCD | the chart always mounts `argocd-styles-cm` at `/shared/app/custom` on argocd-server; `configs.cm` `ui.cssurl: ./custom/argocd.css` loads it (the documented mechanism). `configs.styles` must stay unset, or the chart renders a ConfigMap of the same name |
| Kargo | No stylesheet setting, so the gateway rewrites its `index.html`: SnippetsFilter `kargo/integratn-theme` (`kargo-extras/snippetsfilter.yaml`) adds `<link href="https://argocd.cluster.integratn.tech/custom/kargo.css">` and makes dark the default theme |

argocd-server serves anything under `/shared/app` unauthenticated, which is why
Kargo's stylesheet lives there too: no extra workload, and if ArgoCD is down
Kargo just renders in its own theme.

## How the stylesheets are made

`gen.js` reads a dump of every CSS rule the UI loads and re-emits each colour,
shadow, radius and font declaration under the same selector with `:root `
prepended (one step more specific, so the tool's own hover/active/theme order
still holds), mapped onto the brand tokens by `<tool>/map.js`. `<tool>/base.css`
holds the tokens and what a dump cannot see: colours React writes into `style`
attributes (ArgoCD's status icons, Kargo's stage colours) and SVG `fill`s.
`kargo/extras.js` writes Kargo's `--kargo-color-*` variables and the overrides for
its stage identity palette.

Both themes of each tool map to the same dark look: ArgoCD's CSS carries both
themes in one dump; Kargo's is dumped once per theme (`rules.light`,
`rules.dark`).

## Changing it

Edit a `map.js`, `base.css` or `extras.js`, then:

```bash
hack/ui-themes/build.sh      # rewrites the ConfigMap from the committed dumps
```

## After an ArgoCD or Kargo upgrade

The dumps are tied to the UI version they came from (ArgoCD v3.5.1, Kargo
v1.12.4). After a bump the old selectors mostly still match, but new or renamed
classes come through unstyled. Re-dump from the live UIs (needs `npm i
playwright` and a Chromium):

```bash
cd hack/ui-themes
# ArgoCD: a session token in the argocd.token cookie
COOKIE="argocd.token=$TOKEN" node dump-rules.js https://argocd.cluster.integratn.tech /tmp/argocd.json \
  /applications "/applications?view=list" /applications/argocd/<app>?view=tree /settings /settings/clusters /user-info
gzip -9c /tmp/argocd.json > argocd/rules.json.gz
# Kargo: its bearer token in localStorage, once per theme
for t in light dark; do
  THEME=$t LOCAL="auth_token=$KARGO_TOKEN" node dump-rules.js https://kargo.cluster.integratn.tech /tmp/kargo-$t.json \
    / /project/<project> /project/<project>/stage/<stage> /settings
  gzip -9c /tmp/kargo-$t.json > kargo/rules.$t.json.gz
done
./build.sh
```

Visit every page whose look you care about: a rule only reaches the dump if a
visited page loaded it.

## Known limits

- Kargo's per-stage identity colours (the coloured stage headers and freight
  stripes) are all one neutral, by the brand's "colour only on a verdict" rule.
  Stages are told apart by name.
- Kargo's graph zoom controls and minimap keep their light look in Kargo's light
  theme.
