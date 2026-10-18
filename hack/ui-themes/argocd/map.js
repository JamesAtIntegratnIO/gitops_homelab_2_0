// ArgoCD colours -> IntegratnIO tokens. One dark look whatever ArgoCD's own
// theme setting is: light-theme surfaces map to the same dark tokens.
const T = (n) => `var(--i-${n})`;
const rgb = (s) => s;
const explicit = {
  bg: {
    '255,255,255': T('panel'), '254,254,254': T('panel'), '248,251,251': T('well'), '245,251,253': T('well'),
    '245,245,245': T('well'), '240,240,240': T('well'), '239,243,245': T('well'), '222,230,235': T('ground'),
    '230,230,230': T('raise'), '227,227,227': T('raise'), '224,224,224': T('raise'), '209,213,217': T('raise'),
    '204,214,221': T('raise'), '143,164,177': T('raise'), '153,153,153': T('raise'), '109,127,139': T('raise'),
    '102,102,102': T('raise'), '85,85,85': T('raise'), '73,87,99': T('raise'), '68,68,68': T('raise'),
    '54,60,74': T('raise'), '51,51,51': T('raise'), '48,50,55': T('panel'), '48,48,61': T('well'),
    '40,41,42': T('well'), '38,50,61': T('well'), '34,33,45': T('well'), '25,24,38': T('well'),
    '16,15,15': T('ground'), '15,39,51': T('well'), '10,10,10': T('ground'), '9,9,15': T('ground'), '0,0,0': T('ground'),
  },
  tx: {
    '73,87,99': T('said'), '54,60,74': T('text'), '51,51,51': T('text'), '38,50,61': T('text'), '25,24,38': T('text'),
    '10,10,10': T('text'), '9,9,15': T('text'), '0,0,0': T('text'),
  },
};
explicit.fill = explicit.tx; explicit.stroke = { ...explicit.tx };
function byFamily(kind, fam, L) {
  if (kind === 'bg') {
    if (fam === 'ok') return L > 0.8 ? T('ok-surface') : L < 0.3 ? T('ok-border') : T('ok');
    if (fam === 'warn') return L > 0.8 ? T('warn-surface') : L < 0.3 ? T('warn-surface') : T('warn');
    if (fam === 'bad') return L > 0.8 ? T('bad-surface') : L < 0.15 ? T('bad-surface') : T('bad');
    if (fam === 'accent' || fam === 'aside') return T('raise');
    return L > 0.97 ? T('panel') : L > 0.85 ? T('well') : L > 0.3 ? T('raise') : L > 0.08 ? T('well') : T('ground');
  }
  if (kind === 'bd') {
    if (fam === 'ok') return T('ok-border'); if (fam === 'warn') return T('warn-border'); if (fam === 'bad') return T('bad-border');
    if (fam === 'accent') return T('dim');
    return T('line');
  }
  // text, fill, stroke
  if (fam === 'ok') return T('ok'); if (fam === 'warn') return T('warn'); if (fam === 'bad') return T('bad');
  if (fam === 'accent') return T('text');
  if (fam === 'aside') return T('dim');
  if (L > 0.95) return T('text');
  if (L > 0.75) return T('said');
  if (L > 0.3) return T('dim');
  return T('text');
}
// The few gradients ArgoCD draws, by selector (gen.js drops any not listed).
// The wave is the placeholder shimmer every list shows while it loads, so it
// is the "loading screen" when switching views.
const gradients = {
  '.mockup-list__wave-loader': `linear-gradient(to right, ${T('panel')}, ${T('raise')}, ${T('panel')})`,
  '.expandable--collapsed::after': `linear-gradient(transparent 70%, ${T('panel')} 100%)`,
  '.application-resource-tree--network .application-resource-tree__line': `linear-gradient(90deg, ${T('dim')} 50%, transparent 50%)`,
};
module.exports = {
  explicit, byFamily, pillFrom: 12,
  gradient: (sel) => gradients[sel] || null,
  overlay: (L, a) => (a >= 0.3 ? 'rgba(10, 12, 16, 0.72)' : null),
  font: (v) => (/mono|courier|consol|menlo/i.test(v) ? T('mono') : /heebo|sans|helvetica|arial|roboto|inherit/i.test(v) ? T('sans') : null),
  varKind: (p) => (/toastify-(text|spinner)/.test(p) ? 'tx' : /toastify-color/.test(p) ? 'bg' : null),
  skip: (sel) => /monaco|\.mtk\d|\.xterm/.test(sel),
};
