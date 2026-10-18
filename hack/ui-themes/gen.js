#!/usr/bin/env node
// Generate a brand override stylesheet from a dump of a UI's own CSS rules.
// Every colour-, shadow-, radius- and font-bearing declaration is re-emitted
// under the same selector (one step more specific), mapped onto the
// IntegratnIO tokens. Usage: gen.js <map.js> <rules.json>... > out.css
// Later dumps win where two dumps disagree, so pass the theme you designed
// against last.
const fs = require('fs');
const [,, mapFile, ...rulesFiles] = process.argv;
const rules = rulesFiles.flatMap((f) => JSON.parse(fs.readFileSync(f)));
const M = require(require('path').resolve(mapFile));

// rgb() may nest one level, as Tailwind writes rgb(249 250 251/var(--tw-bg-opacity,1)).
const COLOR_RE = /rgba?\((?:[^()]|\([^()]*\))*\)|#[0-9a-fA-F]{3,8}\b|\b(?:white|black|lightcyan)\b/g;
function parse(c) {
  if (c === 'white') return [255, 255, 255, 1]; if (c === 'black') return [0, 0, 0, 1]; if (c === 'lightcyan') return [224, 255, 255, 1];
  if (c[0] === '#') { let h = c.slice(1); if (h.length <= 4) h = [...h].map(x => x + x).join('');
    return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16), h.length === 8 ? parseInt(h.slice(6, 8), 16) / 255 : 1]; }
  const n = c.match(/[\d.]+/g).map(Number); return [n[0], n[1], n[2], n.length > 3 ? n[3] : 1];
}
// Family of a colour: neutral, accent (the tool's own hue), ok, warn, bad, aside.
function family([r, g, b]) {
  const mx = Math.max(r, g, b), mn = Math.min(r, g, b), ch = (mx - mn) / 255;
  if (ch < 0.16) return 'neutral';
  let h; const d = mx - mn;
  if (mx === r) h = 60 * (((g - b) / d) % 6); else if (mx === g) h = 60 * ((b - r) / d + 2); else h = 60 * ((r - g) / d + 4);
  if (h < 0) h += 360;
  if (h >= 330 || h < 12) return 'bad';
  if (h < 66) return 'warn';
  if (h < 165) return 'ok';
  if (h < 225) return 'accent';
  return 'aside';
}
const lum = ([r, g, b]) => (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255;
const key = ([r, g, b]) => `${r},${g},${b}`;

function mapColor(c, kind) {
  const p = parse(c); const k = key(p);
  if (p[3] === 0) return null;                                  // transparent stays
  const explicit = M.explicit[kind] && M.explicit[kind][k];
  let out = explicit || M.byFamily(kind, family(p), lum(p), p);
  if (!out) return null;
  if (p[3] < 1 && !explicit && kind !== 'bg') out = `color-mix(in srgb, ${out} ${Math.round(p[3] * 100)}%, transparent)`;
  if (p[3] < 1 && !explicit && kind === 'bg') out = M.overlay(lum(p), p[3]) || `color-mix(in srgb, ${out} ${Math.round(p[3] * 100)}%, transparent)`;
  return out;
}
function kindOf(prop) {
  if (prop.startsWith('background')) return 'bg';
  if (prop.startsWith('border') || prop.startsWith('outline') || prop === 'column-rule-color') return 'bd';
  if (prop === 'fill') return 'fill';
  if (prop === 'stroke') return 'stroke';
  if (prop.startsWith('--')) return 'var';
  return 'tx';
}
function scoped(sel) {
  if (M.clean) sel = M.clean(sel);
  // One step more specific than the original, without !important, so the
  // tool's own cascade order (hover, active, theme) is preserved.
  return sel.split(/,(?![^(]*\))/).map(s => {
    s = s.trim();
    if (/^(html|:root)\b/.test(s)) return s.replace(/^(html|:root)/, '$1:root');
    return `:root ${s}`;
  }).join(', ');
}
const out = [];
for (const r of rules) {
  if (M.skip && M.skip(r.sel)) continue;
  const decls = [];
  for (const [prop, val, prio] of r.decls) {
    const imp = prio ? ' !important' : '';
    if (/shadow$/.test(prop) && val !== 'none' && val !== 'initial') { decls.push(`${prop}: none${imp}`); continue; }
    if (/radius$/.test(prop)) { const px = parseFloat(val); if (/px$/.test(val) && px > 0) decls.push(`${prop}: ${px >= M.pillFrom ? '999px' : '6px'}${imp}`); continue; }
    if (prop === 'font-family') { const f = M.font(val); if (f) decls.push(`${prop}: ${f}${imp}`); continue; }
    if (prop === 'background-image' && /gradient/.test(val)) { const g = M.gradient && M.gradient(r.sel, val); if (g) decls.push(`${prop}: ${g}${imp}`); continue; }
    if (!COLOR_RE.test(val)) { COLOR_RE.lastIndex = 0; continue; } COLOR_RE.lastIndex = 0;
    if (/^(background-image|mask|transition|animation)/.test(prop)) continue;
    const kind = prop.startsWith('--') ? M.varKind(prop) : kindOf(prop);
    if (!kind) continue;
    let changed = false;
    const nv = val.replace(COLOR_RE, c => { const m = mapColor(c, kind); if (m) { changed = true; return m; } return c; });
    if (changed) decls.push(`${prop}: ${nv}${imp}`);
  }
  if (!decls.length) continue;
  const block = `${scoped(r.sel)} { ${decls.join('; ')}; }`;
  out.push(r.media ? `@media ${r.media} { ${block} }` : block);
}
// Two dumps (or two pages) repeat most rules; keep each line's last
// occurrence, which is the one that decides the cascade.
const last = new Map(out.map((l, i) => [l, i]));
process.stdout.write(out.filter((l, i) => last.get(l) === i).join('\n') + '\n');
