// Kargo (Ant Design dark algorithm + its own CSS) -> IntegratnIO tokens.
const T = (n) => `var(--i-${n})`;
const explicit = {
  bg: { '20,20,20': T('ground'), '247,248,250': T('ground'), '245,245,245': T('well'), '0,0,0': T('ground'), '31,31,31': T('well'), '40,40,40': T('panel'), '51,51,51': T('panel'),
        '86,86,86': T('line'), '68,68,68': T('line'), '255,255,255': T('panel') },
  tx: { '255,255,255': T('text'), '0,0,0': T('text') },
};
function byFamily(kind, fam, L, p) {
  const a = p[3];
  if (kind === 'bg') {
    if (fam === 'ok') return L < 0.2 ? T('ok-surface') : T('ok');
    if (fam === 'warn') return L < 0.2 ? T('warn-surface') : T('warn');
    if (fam === 'bad') return L < 0.2 ? T('bad-surface') : T('bad');
    if (fam === 'accent' || fam === 'aside') return T('raise');
    if (a < 1) return null;                       // translucent white washes stay as-is
    return L > 0.9 ? T('panel') : L > 0.3 ? T('line') : L > 0.18 ? T('raise') : L > 0.13 ? T('panel') : L > 0.1 ? T('well') : T('ground');
  }
  if (kind === 'bd') {
    if (fam === 'ok') return T('ok-border'); if (fam === 'warn') return T('warn-border'); if (fam === 'bad') return T('bad-border');
    if (fam === 'accent' || fam === 'aside') return T('dim');
    if (a < 1) return null;
    return T('line');
  }
  if (fam === 'ok') return T('ok'); if (fam === 'warn') return T('warn'); if (fam === 'bad') return T('bad');
  if (fam === 'accent') return T('text');
  if (fam === 'aside') return T('dim');
  if (a < 1) return a > 0.75 ? T('text') : a > 0.55 ? T('said') : a > 0.35 ? T('dim') : T('faint');
  if (L > 0.9) return T('text'); if (L > 0.7) return T('said'); if (L > 0.35) return T('dim');
  return T('text');
}
module.exports = {
  explicit, byFamily, pillFrom: 16,
  overlay: (L, a) => (L < 0.1 && a >= 0.4 ? 'rgba(10, 12, 16, 0.72)' : null),
  font: (v) => (/mono|courier|consol|menlo/i.test(v) ? T('mono') : /poppins|sans|helvetica|arial|roboto|system-ui/i.test(v) ? T('sans') : null),
  varKind: () => null,
  clean: (sel) => sel.replace(/:where\(\.css-[\w-]+\)/g, '').replace(/\s+/g, ' ').trim(),
  skip: (sel) => /monaco|\.mtk\d|\.xterm|\.view-lines/.test(sel),
};
