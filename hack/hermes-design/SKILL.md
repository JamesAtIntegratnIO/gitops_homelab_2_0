---
name: integratn-design
description: "Use for any HTML page, report, diagram or visual for James."
version: 1.0.0
author: James (gitops_homelab_2_0)
platforms: [linux]
metadata:
  hermes:
    tags: [Design, HTML, Artifact, Diagram, Report, Brand]
    related_skills: [cluster-health-check, okf-update]
---

# The IntegratnIO design system

James has a design system, "The Night Shift Ledger". Everything visual you
make for him uses it: architecture diagrams, health reports, dashboards,
one-off pages. It is the same system as integratn.io, Specmarshal, Bosun,
the Glance landing page (home.cluster.integratn.tech) and the ArgoCD,
Kargo and authentik themes. Generic "AI dashboard" styling is a miss.

The idea: a graphite ledger read the morning after. Figures in mono,
verdicts said in words, and **nothing coloured unless it is a judgement**.
Because almost nothing is coloured, the one red item reads at once.

## Start from the template, every time

This skill ships two files. Read both before writing any markup:

- `templates/base.css`: the tokens and components. Paste the whole file
  into the page's `<style>`, then add page rules that only spend
  `var(--role)` tokens.
- `templates/page.html`: a finished example (the cluster architecture page)
  showing the header, needs-you callouts, a diagram, tiles and the rail.

Load them with `skill_view` (`file_path: "templates/base.css"`), or read
`/opt/hermes-skills/homelab/integratn-design/templates/`. Copy the
structure and classes; do not restyle them.

The page must be one self-contained HTML file: inline CSS and SVG, no web
fonts, no CDN scripts, no images from the network.

## The rules

**Colour**
- The neutrals carry the page: `--ground` (page), `--well` (sunk: tiles,
  code, diagram field), `--panel` (cards), `--raise` (controls, the one
  emphasised box), `--line` (every border). Text steps: `--text`,
  `--said` (sentences), `--dim` (labels, chrome), `--faint` (asides).
- Green/amber/red (`--ok`, `--warn`, `--bad`) are verdicts only: passed,
  needs a person, broken. Always next to the word that says it ("× cq
  degraded", a `chip bad` reading DEGRADED). Never colour as a category,
  a legend, a zone or decoration.
- Signal orange `#f26a35` is the mark and nothing else.
- No colour literal outside the `:root` block. No other palette (no
  Tailwind slate/emerald/cyan/rose, no purple, no gradients).

**Type**
- System sans for words, system mono for values: ids, IPs, paths, counts,
  versions, dates. A name is sans, its address is mono.
- Nothing below 12px, in the page or in an SVG. Smaller means dimmer, not
  smaller.
- Labels and section heads: 12px, 600, uppercase, letter-spaced, `--dim`
  (`h3` / `.label`). Titles 1rem 600. Page `h1` 1.2rem.
- Numbers tabular and right-aligned in tables.

**Shape and depth**
- One radius, 6px. Pills (999px) only for chips.
- Flat: depth is one neutral step plus a 1px hairline. No box-shadow, no
  glow, no translucency, no backdrop blur.
- No animation (no pulsing dots), no background grids, no emoji.

**Saying things**
- A value you could not read says "not recorded" in `--dim` where the
  number would be. Never 0, never an error box.
- Lead with what needs James. One summary line under the `h1`: the
  judgement first in its verdict colour ("2 degraded"), then the counts in
  dim, separated by faint dots. If nothing needs him, say "Nothing needs
  you", never "0 issues".
- Anything that needs him gets a callout directly under the header:
  `callout bad|warn` led by a verdict chip, one sentence, a link to the
  exact thing (the PR, the app, the dashboard).
- At most one primary button (`btn primary`) per section; the rest `btn`.

**Layout**
- `.page` (max 80rem), then `.board`: a work column plus a 21rem rail of
  reference panels that drops below under 68rem. Tiles in `.tiles`.
- Works at 390px wide with no sideways page scroll; wide things scroll in
  their own box.
- Dark is the default. base.css already switches to the light ramp for a
  viewer whose system is light; do not add a theme of your own.

## Diagrams

Draw them as inline SVG inside `<figure class="diagram">`, using the
diagram classes in base.css. Copy the shapes from page.html.

- Give the `<svg>` `width` and `height` equal to its `viewBox`, at most
  1200 wide, and put it full width (outside the board). Drawn 1:1, 12px
  text stays 12px; a scaled-down SVG breaks the floor rule.
- Boxes: `g.node` (panel fill, hairline). `node raise` for the one box to
  notice, `node ext` (dashed) for things outside James's house. `node
  warn|bad` only for a box that is itself in that state.
- Grouping is structure, not colour: `g.group` is a hairline frame with a
  `text.gl` label (uppercase, dim); `group zone` is dashed for a network or
  trust boundary. Put frame labels where no edge crosses them.
- Text: `.t` title (13px 600), `.s` detail in mono 12px, at most ~28
  characters a line in a 215px box, `.v-ok|v-warn|v-bad` for a verdict line
  ("× cq degraded", "· EOL (accepted risk)").
- Edges: `path.edge` (faint), `edge soft` dashed for control or side
  traffic (DNS, alerts, secrets), `edge bad` only for a broken path. One
  arrow marker, `.el` labels in 12px. Route edges through the gaps between
  boxes, never through a box.
- No legend of colours. If a shape needs explaining, label it in place.

## What went wrong last time

The first architecture page Hermes made (2026-11-08) was rejected as
"standard AI slop". It had: JetBrains Mono from Google Fonts for every
word, Tailwind slate backgrounds with translucent cards, a different neon
fill for every box category plus a colour legend, a pulsing green dot,
a background grid, 8px SVG text, and red text used for things that were
not broken. Do none of that. page.html is the same content done right.

## Before you hand it over

Check the file, then fix and recheck:
1. `grep -o '#[0-9a-fA-F]\{3,6\}'`: every hit is inside `:root`, the
   light-mode block, or the mark's `#f26a35`/`#fff`.
2. No `font-size` under 12px, no `font-size="8"` / `"10"` in SVG.
3. No `box-shadow`, `animation`, `@keyframes`, `rgba(`, `gradient`,
   `fonts.googleapis`, `<script src`, `<link`.
4. Every coloured word is a verdict and says it in words.
5. If you can render it (the browser toolset), screenshot it at 1400 and
   390 wide, dark and light, and look before sending.

If the design system itself needs a change, don't improvise one in a page:
propose it with skill-improve. The source of truth is
`DESIGN.md` in IntegratnIO/specmarshal; this skill's files live in
`hack/hermes-design/` in gitops_homelab_2_0 and are generated into
`addons/cluster-roles/control-plane/addons/hermes/design-skill.yaml`
by `hack/hermes-design/gen.sh`.
