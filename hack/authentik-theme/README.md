# authentik in the IntegratnIO brand

authentik (`auth.cluster.integratn.tech`) wears the same design system as the
landing page, ArgoCD and Kargo (`hack/ui-themes/`): graphite ground, flat
hairline panels, colour only on a verdict, system sans and mono, 6px radius,
signal orange only in the mark. Its flow pages get backgrounds redrawn from the
cyberdeck wallpapers in nix_dots_2.

## What it sets

| Piece | Where it ends up |
|---|---|
| Title, logo, favicon, custom CSS, default flow background, dark theme, user-interface background | the **default brand**, via blueprint `10-integratn-brand.yaml` in ConfigMap `authentik-blueprints-brand` |
| A background per default flow (table below) | the same blueprint, `background` on each flow; entries are skipped for flows that do not exist |
| The SVGs (mark, lockup, six backgrounds) | ConfigMap `authentik-brand-assets`, mounted on authentik-server at `/web/dist/assets/images/integratn`, served as `/static/dist/assets/images/integratn/<file>` |

Both ConfigMaps live in `addons/clusters/the-cluster/addons/authentik/` and are
**generated**; the mount and the blueprint ConfigMap are wired in
`addons/cluster-roles/control-plane/addons/authentik/values.yaml`.

The blueprint finds the brand by `default: true`, not by domain, and only
writes branding fields, so the brand's flows and certificates stay as they
are. It does replace the brand's `attributes` as a whole.

## Backgrounds

`scenes.js` ports `modules/nixos/profiles/cyberdeck/wallpaper.nix` and
`wallpaper-scenes.nix` from nix_dots_2: the same scenes and geometry on a
1920x1080 canvas, minus the neon, glow and scanlines, in the brand's greys. The busiest
parts sit in the outer thirds and the lower band, clear of the card. Random
numbers come from SHA-256 of a key, so a rebuild gives the same files.

| Scene | Flows |
|---|---|
| grid (horizon + dot field) | brand default: every flow not listed, and the user interface |
| ridgeline | `default-authentication-flow` |
| traces (an "OIDC" chip) | `default-provider-authorization-{implicit,explicit}-consent` |
| sweep (radar, hosts as contacts) | `default-invalidation-flow`, `default-provider-invalidation-flow` |
| skyline | `default-source-{authentication,enrollment,pre-authentication}` |
| hexdump | `default-password-change`, `default-authenticator-{totp,webauthn,static}-setup` |

authentik hides the background below 35rem wide; on a phone the card fills the
screen.

## Changing it

Edit `brand.css`, `scenes.js` or the flow table in `gen.js`, then:

```bash
hack/authentik-theme/build.sh
```

The mark files in `mark/` are copies of integratn.io's
(IntegratnIO/integratn-io-astro `public/favicon.svg` and
`public/brand/integratn-mark.svg`).

## How the CSS works

authentik puts the brand CSS in the page `<head>` and adopts it into every
component's shadow root. The token block overrides PatternFly's global
variables (doubled `html[data-theme][data-theme]` to beat the dark theme's
`html[data-theme="dark"]`), which reach every component by inheritance; the
component rules match inside each component.

One trap: the flow background is `body::before` at `z-index: -1`. Giving
`<html>` a background stops body's background from propagating to the canvas,
and body's own then paints over the image. Only `body` gets the ground.

## Checking it locally

Written against authentik 2026.2.3 run from `authentik/server:2026.2.3`
(Docker Hub; ghcr's blob host is blocked in the agent sandbox) with Postgres,
this repo's blueprints and both ConfigMaps mounted as the chart mounts them,
then Playwright screenshots of the login, consent, password change, enrollment,
user and admin pages.

## Known limits

- The admin dashboard's charts are canvas with colours hard-coded in authentik
  (`EventChart.ts`), so they keep authentik's blues, greens and orange.
- The initials avatar keeps authentik's generated colour.
- After an authentik upgrade, re-check the login page and the user interface:
  the CSS leans on PatternFly variables and a few authentik parts
  (`body:has(ak-flow-executor)`, `.pf-c-login__main`).
