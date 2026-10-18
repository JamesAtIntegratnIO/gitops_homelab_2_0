#!/usr/bin/env node
// Kargo pieces the stylesheet dump cannot see: the --kargo-color-* variables it
// writes inline on <html>, and the colours React writes into style attributes
// (stage identity colours, health icons, graph edges).
const vars = {
  'bg-layout': 'ground', 'bg-base': 'ground', 'bg-container': 'panel', 'bg-elevated': 'raise', 'bg-spotlight': 'raise',
  'border': 'line', 'border-secondary': 'line', 'split': 'line',
  'text': 'text', 'text-base': 'text', 'text-heading': 'text', 'text-label': 'dim', 'text-description': 'dim',
  'text-secondary': 'said', 'text-tertiary': 'dim', 'text-quaternary': 'faint', 'text-placeholder': 'faint', 'text-disabled': 'faint',
  'link': 'text', 'link-hover': 'said', 'link-active': 'said',
  'primary': 'said', 'primary-hover': 'text', 'primary-active': 'said', 'primary-text': 'text', 'primary-text-hover': 'text', 'primary-text-active': 'said',
  'primary-bg': 'raise', 'primary-bg-hover': 'raise', 'primary-border': 'line', 'primary-border-hover': 'dim',
  'info': 'said', 'info-text': 'said', 'info-bg': 'raise', 'info-border': 'line',
};
for (const s of ['success', 'warning', 'error']) {
  const t = { success: 'ok', warning: 'warn', error: 'bad' }[s];
  Object.assign(vars, { [s]: t, [`${s}-text`]: t, [`${s}-text-hover`]: t, [`${s}-text-active`]: t, [`${s}-hover`]: t, [`${s}-active`]: t,
    [`${s}-bg`]: `${t}-surface`, [`${s}-bg-hover`]: `${t}-surface`, [`${s}-border`]: `${t}-border`, [`${s}-border-hover`]: `${t}-border` });
}
const out = [':root {'];
for (const [k, v] of Object.entries(vars)) out.push(`  --kargo-color-${k}: var(--i-${v}) !important;`);
out.push('}');
// Stage identity palettes (features/stage/utils/index.ts). Colour only on a
// verdict, so every identity colour becomes the same neutral.
const palette = ['#ED204E', '#FD5352', '#FE7537', '#e78a00', '#DFC546', '#9bce22', '#84DF75', '#1CAC77', '#1bc1a7', '#1DCECA', '#0DAFD3', '#3882EA', '#2D5EDC', '#6380E1', '#7851AA', '#A9499D', '#D0469D', '#E573A2', '#f1619b', '#FE43A3', '#6a7382',
  '#D70015', '#C93500', '#B24F01', '#248A3D', '#0E817C', '#028299', '#0471A4', '#013fDC', '#3634A3', '#8944AA', '#C1236B', '#D30F45', '#6b7280'];
const rgb = (h) => `rgb(${parseInt(h.slice(1, 3), 16)}, ${parseInt(h.slice(3, 5), 16)}, ${parseInt(h.slice(5, 7), 16)})`;
const cols = palette.map(rgb);
const sel = (fmt) => cols.map((c) => fmt(c)).join(',\n');
out.push(sel((c) => `[style*="background-color: ${c}"], [style*="background: ${c}"], [style*=", ${c};"]`) + ' { background: var(--i-raise) !important; }');
out.push(sel((c) => `[style*="border: 1px solid ${c}"], [style*="border-color: ${c}"], [style*="border-left: 4px solid ${c}"]`) + ' { border-color: var(--i-line) !important; }');
out.push(sel((c) => `[style^="color: ${c}"], [style*="; color: ${c}"]`) + ' { color: var(--i-said) !important; }');
out.push(sel((c) => `[style*="stroke: ${c}"]`) + ' { stroke: var(--i-dim) !important; }');
// Health icons (features/common/health-status/health-status-icon.tsx).
out.push('[style^="color: rgb(82, 196, 26)"], [style*="; color: rgb(82, 196, 26)"] { color: var(--i-ok) !important; }');
out.push('[style^="color: rgb(245, 34, 45)"], [style*="; color: rgb(245, 34, 45)"] { color: var(--i-bad) !important; }');
out.push('[style^="color: rgb(250, 173, 20)"], [style*="; color: rgb(250, 173, 20)"] { color: var(--i-warn) !important; }');
out.push('[style^="color: rgb(13, 171, 234)"], [style*="; color: rgb(13, 171, 234)"] { color: var(--i-said) !important; }');
out.push('[style*="stroke: rgb(156, 163, 175)"] { stroke: var(--i-line) !important; }');
out.push('[style*="text-shadow"] { text-shadow: none !important; }');
process.stdout.write(out.join('\n') + '\n');
