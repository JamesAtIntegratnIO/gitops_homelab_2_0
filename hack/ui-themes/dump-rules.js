#!/usr/bin/env node
// Dump every CSS rule a UI loads, as JSON, by walking its pages in Chromium.
// The input to gen.js. Needs `npm i playwright` (any recent version) and a
// Chromium; see README.md for the auth each UI wants.
//
//   node dump-rules.js <base-url> <out.json> <path>...
//
// Env: CHROME=<chromium binary> (optional), COOKIE=name=value (ArgoCD session),
//      LOCAL=key=value (Kargo token in localStorage), THEME=dark|light (the
//      UI's own theme to dump; default dark).
const { chromium } = require('playwright');
const fs = require('fs');

(async () => {
  const [, , base, out, ...paths] = process.argv;
  const browser = await chromium.launch({ executablePath: process.env.CHROME || undefined });
  const theme = process.env.THEME || 'dark';
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, colorScheme: theme });
  if (process.env.COOKIE) {
    const [name, ...v] = process.env.COOKIE.split('=');
    await ctx.addCookies([{ name, value: v.join('='), url: base }]);
  }
  await ctx.addInitScript(([kv, t]) => {
    try {
      localStorage.setItem('theme-mode', t);
      if (kv) { const [k, ...v] = kv.split('='); localStorage.setItem(k, v.join('=')); }
    } catch (e) { /* no storage */ }
  }, [process.env.LOCAL || '', theme]);
  const page = await ctx.newPage();
  const seen = new Map();
  for (const p of paths) {
    await page.goto(base + p);
    await page.waitForTimeout(3000);
    const rules = await page.evaluate(() => {
      const found = [];
      const walk = (list, media) => {
        for (const r of list) {
          if (r instanceof CSSMediaRule) walk(r.cssRules, (media ? media + ' and ' : '') + r.conditionText);
          else if (r instanceof CSSSupportsRule) walk(r.cssRules, media);
          else if (r instanceof CSSStyleRule) {
            const decls = [];
            for (let i = 0; i < r.style.length; i++) {
              const k = r.style[i];
              decls.push([k, r.style.getPropertyValue(k), r.style.getPropertyPriority(k)]);
            }
            found.push({ media, sel: r.selectorText, decls });
            if (r.cssRules && r.cssRules.length) walk(r.cssRules, media);
          }
        }
      };
      for (const s of document.styleSheets) {
        // Skip our own stylesheet, or a second run would remap its own output.
        if (s.href && /\/custom\/|integratn/.test(s.href)) continue;
        try { walk(s.cssRules, ''); } catch (e) { /* cross-origin sheet */ }
      }
      return found;
    });
    for (const r of rules) {
      const k = r.media + '|' + r.sel + '|' + JSON.stringify(r.decls);
      if (!seen.has(k)) seen.set(k, r);
    }
    console.error(`${p}: ${seen.size} rules so far`);
  }
  fs.writeFileSync(out, JSON.stringify([...seen.values()]));
  await browser.close();
})();
