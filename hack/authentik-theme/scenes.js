// Flow backgrounds for authentik, in the IntegratnIO brand.
//
// The scenes are the cyberdeck wallpapers from nix_dots_2
// (modules/nixos/profiles/cyberdeck/wallpaper.nix and wallpaper-scenes.nix):
// horizon grid, ridgeline, board traces, radar sweep, hexdump and skyline.
// Same geometry, redrawn for the brand: graphite ground, hairlines in the
// line/faint greys, no neon, no glow, no scanlines. Colour is kept for
// verdicts, so nothing here is coloured; the only orange on the page is the
// mark on the card.
//
// Each scene is drawn on a 1920x1080 canvas and scales with `slice`, the same
// as CSS `cover`. The login card sits in the middle column, so the busy parts
// of each scene stay in the outer thirds and the lower band. Random numbers come
// from SHA-256 of a key, as in the Nix version, so a build is reproducible.
"use strict";
const crypto = require("crypto");

const W = 1920;
const H = 1080;
const HORIZON = 760;

const c = {
  ground: "#0a0c10", well: "#14171d", panel: "#1d2126", raise: "#262a30", line: "#393e43",
  text: "#f3f7fc", said: "#cfd5dc", dim: "#acb2b9", faint: "#8d939a",
};
const MONO = "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace";

const r1 = (x) => Math.round(x * 10) / 10;
const hash = (k) => crypto.createHash("sha256").update(k).digest("hex");
// The k-th number in [0, 1) from a hash (k = 0..9).
const rand = (h, k) => parseInt(h.substr(k * 6, 6), 16) / 16777216;
const gauss = (u) => Math.exp(-u * u);
const lerp = (a, b, t) => a + (b - a) * t;
// Mix two #rrggbb colours.
const mix = (a, b, t) => {
  const ch = (h, i) => parseInt(h.substr(1 + i * 2, 2), 16);
  return "#" + [0, 1, 2].map((i) => Math.round(lerp(ch(a, i), ch(b, i), t)).toString(16).padStart(2, "0")).join("");
};

// Perspective floor: horizontal lines bunch up toward the horizon, rays fan
// out from a vanishing point just above it.
const floorLines = () => {
  let out = "";
  for (let k = 1; k <= 18; k++) {
    const y = HORIZON + k * k * 1.0;
    if (y > H) break;
    out += `<line x1="0" y1="${r1(y)}" x2="${W}" y2="${r1(y)}"/>`;
  }
  return out;
};
const rays = () => {
  let out = "";
  for (let i = -20; i <= 20; i++) out += `<line x1="${W / 2}" y1="${HORIZON - 60}" x2="${W / 2 + i * 260}" y2="${H}"/>`;
  return out;
};

// L-shaped corner marks and the four corner labels: the HUD furniture of the
// original, in mono at 14px (the brand floor is 12px, and the canvas is
// usually shown a little smaller than 1920 wide).
const hud = (label) => {
  // Inset far enough that a 4:3 screen, which crops the sides, still shows them.
  const L = 260, R = W - 260, T = 40, B = H - 40;
  const corner = (x, y, dx, dy) => `<path d="M${x} ${y + dy * 22}L${x} ${y}L${x + dx * 22} ${y}"/>`;
  return `
  <g stroke="${c.line}" stroke-width="1.5" fill="none">
    ${corner(L, T, 1, 1)}${corner(R, T, -1, 1)}${corner(L, B, 1, -1)}${corner(R, B, -1, -1)}
  </g>
  <g font-family="${MONO}" font-size="14" letter-spacing="1.5" fill="${c.faint}" fill-opacity="0.8">
    <text x="${L + 28}" y="${B - 16}">${label}</text>
    <text x="${R - 28}" y="${B - 16}" text-anchor="end">AUTHORIZED USE ONLY</text>
  </g>`;
};

const svg = (scene, label) => `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}" preserveAspectRatio="xMidYMid slice">
  <defs>
    <radialGradient id="bg" cx="50%" cy="42%" r="75%">
      <stop offset="0" stop-color="${c.well}"/>
      <stop offset="1" stop-color="${c.ground}"/>
    </radialGradient>
    <pattern id="dots" width="24" height="24" patternUnits="userSpaceOnUse">
      <circle cx="12" cy="12" r="1.1" fill="${c.faint}" fill-opacity="0.35"/>
    </pattern>
    <linearGradient id="floorFade" x1="0" y1="${HORIZON}" x2="0" y2="${H}" gradientUnits="userSpaceOnUse">
      <stop offset="0" stop-color="#fff" stop-opacity="0"/>
      <stop offset="1" stop-color="#fff" stop-opacity="1"/>
    </linearGradient>
    <mask id="floorMask"><rect y="${HORIZON}" width="${W}" height="${H - HORIZON}" fill="url(#floorFade)"/></mask>
    <linearGradient id="dotFade" x1="0" y1="0" x2="0" y2="${HORIZON}" gradientUnits="userSpaceOnUse">
      <stop offset="0" stop-color="#fff" stop-opacity="0.9"/>
      <stop offset="1" stop-color="#fff" stop-opacity="0.1"/>
    </linearGradient>
    <mask id="dotMask"><rect width="${W}" height="${HORIZON}" fill="url(#dotFade)"/></mask>
    <linearGradient id="horizonLine" x1="0" x2="1">
      <stop offset="0" stop-color="${c.faint}" stop-opacity="0"/>
      <stop offset="0.5" stop-color="${c.faint}" stop-opacity="0.8"/>
      <stop offset="1" stop-color="${c.faint}" stop-opacity="0"/>
    </linearGradient>${scene.defs || ""}
  </defs>
  <rect width="${W}" height="${H}" fill="url(#bg)"/>
  ${scene.body}
  ${hud(label)}
</svg>
`;

// --- grid: dot field above a perspective floor -------------------------------
const grid = () => ({
  body: `
  <rect width="${W}" height="${HORIZON}" fill="url(#dots)" mask="url(#dotMask)"/>
  <g mask="url(#floorMask)" stroke="${c.line}" stroke-width="1">${floorLines()}${rays()}</g>
  <rect y="${HORIZON}" width="${W}" height="1" fill="url(#horizonLine)"/>`,
});

// --- ridgeline: stacked terrain profiles, back to front ----------------------
// Peaks sit in the outer thirds; the middle stays low, under the card.
const ridgeline = () => {
  const n = 26;
  let body = "";
  for (let i = 0; i < n; i++) {
    const t = i / (n - 1);
    const y0 = 470 + t * 580;
    const hr = hash(`ridge:${i}`);
    const ph = rand(hr, 0) * 2 * Math.PI;
    const ph2 = rand(hr, 1) * 2 * Math.PI;
    let d = `M-10 ${r1(y0)}`;
    for (let x = 0; x <= W + 10; x += 8) {
      const hp = hash(`ridge:${i}:${x}`);
      const env = gauss((x - 330) / 260) + gauss((x - 1600) / 280) + 0.35 * gauss((x - 960) / 520);
      const h = env * (Math.abs(Math.sin(x * 0.011 + ph)) * 70 + Math.sin(x * 0.034 + ph2) * 16 + rand(hp, 0) * 10) * (0.5 + t * 0.9) + rand(hp, 1) * 2;
      d += `L${x} ${r1(y0 - h)}`;
    }
    d += `L${W + 10} ${H + 10}L-10 ${H + 10}Z`;
    // Filled, so each profile hides the ones behind it.
    body += `<path d="${d}" fill="${c.ground}" stroke="${mix(c.line, c.faint, t)}" stroke-opacity="${r1(0.35 + t * 0.5)}" stroke-width="1.2"/>\n`;
  }
  return { body: `<rect width="${W}" height="${H}" fill="url(#dots)" opacity="0.6"/>\n${body}` };
};

// --- traces: board traces fanning out of a chip -------------------------------
const traces = (chip, sub) => {
  const cx = 1560, cy = 470, sz = 110;
  const sides = [[1, 0], [-1, 0], [0, 1], [0, -1]];
  let paths = "", vias = "";
  for (const [dx, dy] of sides) {
    for (let i = 0; i < 11; i++) {
      const h = hash(`trace:${dx}:${dy}:${i}`);
      const off = -sz + 18 + i * ((2 * sz - 36) / 10);
      const x = cx + dx * sz + (dx !== 0 ? 0 : off);
      const y = cy + dy * sz + (dy !== 0 ? 0 : off);
      // Short toward the card on the left, longer out to the right edge.
      const reach = dx < 0 ? 150 : dx > 0 ? 300 : dy > 0 ? 330 : 260;
      const k = (i - 5) * 22;
      const a = Math.abs(k);
      const l1 = 20 + (5 - Math.abs(i - 5)) * 10 + rand(h, 0) * 10;
      const l3 = 30 + rand(h, 1) * (reach - 90);
      const qx = dx !== 0 ? 0 : 1, qy = dx !== 0 ? 1 : 0;
      const x1 = x + dx * l1, y1 = y + dy * l1;
      const x2 = x1 + dx * a + qx * k, y2 = y1 + dy * a + qy * k;
      const x3 = x2 + dx * l3, y3 = y2 + dy * l3;
      const lit = rand(h, 2) < 0.25;
      const col = lit ? c.faint : c.line;
      paths += `<path d="M${r1(x)} ${r1(y)}L${r1(x1)} ${r1(y1)}L${r1(x2)} ${r1(y2)}L${r1(x3)} ${r1(y3)}" stroke="${col}"/>`;
      vias += `<circle cx="${r1(x3)}" cy="${r1(y3)}" r="4" fill="${c.ground}" stroke="${col}"/>`;
    }
  }
  return {
    body: `
  <rect width="${W}" height="${H}" fill="url(#dots)" opacity="0.7"/>
  <g fill="none" stroke-width="1.6" stroke-linejoin="round">${paths}</g>
  <g stroke-width="1.6">${vias}</g>
  <rect x="${cx - sz}" y="${cy - sz}" width="${2 * sz}" height="${2 * sz}" rx="6" fill="${c.panel}" stroke="${c.faint}" stroke-width="1.5"/>
  <rect x="${cx - sz + 14}" y="${cy - sz + 14}" width="${2 * sz - 28}" height="${2 * sz - 28}" rx="4" fill="none" stroke="${c.line}"/>
  <circle cx="${cx - sz + 30}" cy="${cy - sz + 30}" r="4" fill="${c.faint}"/>
  <text x="${cx}" y="${cy + 8}" text-anchor="middle" font-family="${MONO}" font-weight="700" font-size="34" letter-spacing="4" fill="${c.said}">${chip}</text>
  <text x="${cx}" y="${cy + 40}" text-anchor="middle" font-family="${MONO}" font-size="14" letter-spacing="2" fill="${c.faint}">${sub}</text>`,
  };
};

// --- sweep: a radar scope with the cluster's hosts as contacts ----------------
const sweep = (peers) => {
  const cx = 1520, cy = 520, r = 360;
  const at = (deg, dist) => ({ x: cx + dist * Math.cos((deg * Math.PI) / 180), y: cy + dist * Math.sin((deg * Math.PI) / 180) });
  const rot = (deg) => `transform="rotate(${deg} ${cx} ${cy})"`;
  let rings = "";
  for (const q of [90, 180, 270, 360]) rings += `<circle cx="${cx}" cy="${cy}" r="${q}" fill="none" stroke="${q === r ? c.faint : c.line}" stroke-opacity="${q === r ? 0.6 : 1}"/>`;
  let ticks = "";
  for (let i = 0; i < 72; i++) {
    const big = i % 6 === 0;
    ticks += `<line x1="${cx + r}" y1="${cy}" x2="${cx + r + (big ? 16 : 7)}" y2="${cy}" stroke="${big ? c.faint : c.line}" ${rot(i * 5)}/>`;
  }
  // The afterglow: sixteen 4-degree wedges behind the sweep line, fading out.
  const edge = at(-4, r);
  let wedges = "";
  for (let i = 0; i < 16; i++) {
    wedges += `<path d="M${cx} ${cy}L${cx + r} ${cy}A${r} ${r} 0 0 0 ${r1(edge.x)} ${r1(edge.y)}Z" fill="${c.faint}" fill-opacity="${r1(0.12 * (1 - i / 16) * 100) / 100}" ${rot(-40 - i * 4)}/>`;
  }
  const blip = (deg, dist, name, col) => {
    const p = at(deg, dist);
    return `<circle cx="${r1(p.x)}" cy="${r1(p.y)}" r="12" fill="${col}" fill-opacity="0.12"/><circle cx="${r1(p.x)}" cy="${r1(p.y)}" r="4.5" fill="${col}"/>` +
      (name ? `<text x="${r1(p.x + 18)}" y="${r1(p.y + 5)}" font-family="${MONO}" font-size="15" letter-spacing="1.5" fill="${c.dim}">${name}</text>` : "");
  };
  const slots = [[-95, 240], [-150, 300], [160, 210], [75, 280], [20, 150]];
  return {
    body: `
  <rect width="${W}" height="${H}" fill="url(#dots)" opacity="0.5"/>
  ${rings}${ticks}
  <path d="M${cx - r} ${cy}H${cx + r}M${cx} ${cy - r}V${cy + r}" stroke="${c.line}"/>
  ${wedges}
  <line x1="${cx}" y1="${cy}" x2="${cx + r}" y2="${cy}" stroke="${c.dim}" stroke-width="2" ${rot(-40)}/>
  ${peers.map((n, i) => blip(slots[i][0], slots[i][1], n, c.said)).join("")}
  ${blip(118, 300, "", c.line)}${blip(-12, 330, "", c.line)}
  <circle cx="${cx}" cy="${cy}" r="5" fill="${c.text}"/>`,
  };
};

// --- hexdump: a memory dump with a readable row, fading in from the left ------
const hexdump = (label) => {
  const x0 = 70, size = 15, lineH = 24, rows = 36, marked = 15;
  const text = (label + " ".repeat(16)).slice(0, 16);
  const group = (b) => `${b.slice(0, 8).join(" ")}  ${b.slice(8).join(" ")}`;
  const ascii = (v) => (v >= 97 && v <= 122) || (v >= 48 && v <= 57) ? String.fromCharCode(v) : ".";
  let out = "";
  for (let i = 0; i < rows; i++) {
    const h = hash(`hexdump:${i}`);
    const bytes = Array.from({ length: 16 }, (_, j) => (parseInt(h.substr(32 + j * 2, 2), 16) < 51 ? "00" : h.substr(j * 2, 2)));
    const addr = (0x4a20 + i * 16).toString(16).padStart(8, "0");
    const y = 110 + i * lineH;
    if (i === marked) {
      const hex = [...text].map((ch) => ch.charCodeAt(0).toString(16).padStart(2, "0"));
      out += `<text x="${x0}" y="${y}" xml:space="preserve" fill="${c.faint}">${addr}  <tspan fill="${c.said}">${group(hex)}</tspan>  |<tspan fill="${c.text}">${text}</tspan>|</text>\n`;
    } else {
      out += `<text x="${x0}" y="${y}" xml:space="preserve">${addr}  ${group(bytes)}  |${bytes.map((b) => ascii(parseInt(b, 16))).join("")}|</text>\n`;
    }
  }
  // Mirrored on the right, offset, so the card has dump on both sides.
  return {
    defs: `
    <linearGradient id="dumpFade" x1="0" x2="1">
      <stop offset="0" stop-color="#fff" stop-opacity="1"/>
      <stop offset="0.36" stop-color="#fff" stop-opacity="0.15"/>
      <stop offset="0.64" stop-color="#fff" stop-opacity="0.15"/>
      <stop offset="1" stop-color="#fff" stop-opacity="1"/>
    </linearGradient>
    <mask id="dumpMask"><rect width="${W}" height="${H}" fill="url(#dumpFade)"/></mask>`,
    body: `
  <g mask="url(#dumpMask)" font-family="${MONO}" font-size="${size}" fill="${c.line}" fill-opacity="0.75">
  ${out}
  <g transform="translate(${W - 2 * x0 - 640} ${lineH / 2})">${out.replace(new RegExp(`fill="${c.text}"|fill="${c.said}"`, "g"), `fill="${c.line}"`)}</g>
  </g>`,
  };
};

// --- skyline: a night skyline standing on the grid's horizon ------------------
const skyline = () => {
  const street = (start, count, mk) => {
    let x = start, out = "";
    for (let i = 0; i < count && x < W; i++) {
      const b = mk(i, x);
      out += b.svg;
      x += b.advance;
    }
    return out;
  };
  // Tall towers in the outer thirds, low blocks under the card.
  const tall = (x) => gauss((x - 330) / 230) + gauss((x - 1600) / 260);
  const back = street(-10, 120, (i, x) => {
    const h = hash(`sky:back:${i}`);
    const w = 30 + rand(h, 0) * 50;
    const ht = 50 + rand(h, 1) * 110 + 120 * tall(x);
    return { advance: w + rand(h, 2) * 6, svg: `<rect x="${r1(x)}" y="${r1(HORIZON - ht)}" width="${r1(w)}" height="${r1(ht)}"/>` };
  });
  const front = street(-6, 90, (i, x) => {
    const h = hash(`sky:front:${i}`);
    const w = 44 + Math.floor(rand(h, 0) * 5) * 12;
    const ht = 30 + rand(h, 1) * 80 + 300 * rand(h, 2) * tall(x);
    const top = HORIZON - ht;
    const mid = x + w / 2;
    let win = "";
    const cols = Math.floor((w - 18) / 12) + 1;
    const rowsN = Math.floor((ht - 20) / 14) + 1;
    for (let col = 0; col < cols; col++) for (let row = 0; row < rowsN; row++) {
      const hw = hash(`sky:win:${i}:${col}:${row}`);
      if (rand(hw, 0) < 0.22) win += `<rect x="${r1(x + 9 + col * 12)}" y="${r1(top + 10 + row * 14)}" width="5" height="7" fill="${c.faint}" fill-opacity="${r1(0.25 + rand(hw, 1) * 0.5)}" stroke="none"/>`;
    }
    const mast = ht > 220 ? `<path d="M${r1(mid)} ${r1(top)}v-26"/><circle cx="${r1(mid)}" cy="${r1(top - 28)}" r="2.4" fill="${c.dim}" stroke="none"/>` : "";
    return { advance: w + 3 + rand(h, 3) * 16, svg: `<rect x="${r1(x)}" y="${r1(top)}" width="${w}" height="${r1(ht)}"/>${mast}${win}` };
  });
  return {
    body: `
  <rect width="${W}" height="${HORIZON}" fill="url(#dots)" mask="url(#dotMask)" opacity="0.6"/>
  <g fill="${c.panel}" fill-opacity="0.6">${back}</g>
  <g fill="${c.ground}" stroke="${c.line}" stroke-width="1.2">${front}</g>
  <rect y="${HORIZON}" width="${W}" height="${H - HORIZON}" fill="${c.ground}"/>
  <g mask="url(#floorMask)" stroke="${c.line}" stroke-width="1">${floorLines()}</g>
  <rect y="${HORIZON}" width="${W}" height="1.5" fill="url(#horizonLine)"/>`,
  };
};

module.exports = { W, H, c, svg, grid, ridgeline, traces, sweep, hexdump, skyline };
