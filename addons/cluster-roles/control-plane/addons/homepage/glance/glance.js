try { document.documentElement.dataset.page = pageData.slug; } catch (e) {}
/* Needs you loads after the page, so a #section or #alert-X link would
   land before its target exists. Scroll once it appears. */
if (location.hash.length > 1) {
  const id = location.hash.slice(1), t0 = Date.now();
  const t = setInterval(() => {
    const el = document.getElementById(id);
    if (el) { el.scrollIntoView({ block: "start" }); clearInterval(t); }
    else if (Date.now() - t0 > 10000) clearInterval(t);
  }, 150);
}
