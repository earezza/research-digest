// On-device engine for the mobile app. Same sources and ranking as the desktop program (see catalog.json),
// running inside the app. Inside Capacitor, fetch goes through the phone's native HTTP stack, so the
// research sources work without a server. No keys or accounts: Europe PMC, bioRxiv, medRxiv and arXiv are open.

let C = null;
async function catalog() {
  if (!C) {
    // The app bundles catalog.js (written at build time); elsewhere the JSON is fetched.
    const c = await import("./catalog.js").then(m => structuredClone(m.default)).catch(async () => (await fetch("catalog.json")).json());
    c.issn = new Map(c.journals.map(([name, issn, tier, field]) => [issn.toUpperCase(), { name, issn: issn.toUpperCase(), tier, field }]));
    c.days = Object.fromEntries(c.windows.map(w => [w.id, w.days]));
    c.labels = Object.fromEntries(c.windows.map(w => [w.id, w.label]));
    c.rules = c.fieldRules.map(([f, re]) => [f, new RegExp(re, "i")]);
    c.cueRes = c.cues.map(re => new RegExp(re, "i"));
    c.junk = new RegExp(c.junkTitle, "i");
    c.key = new RegExp(c.keySentence, "i");
    // Section names of structured abstracts (same handling as the desktop program's abstract.go).
    c.secOf = new Map(); c.canon = new Map();
    const names = [];
    for (const [part, ns] of Object.entries(c.abstractSections || {})) for (const n of ns) { c.secOf.set(n.toLowerCase(), part); c.canon.set(n.toLowerCase(), n); names.push(n); }
    names.sort((a, b) => b.length - a.length);
    const esc = x => x.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const alt = names.flatMap(n => [n, n.toUpperCase(), n[0] + n.slice(1).toLowerCase()]).map(esc).join("|");
    c.plainHead = new RegExp(`(^|[.!?]\\s+|\\s)(${alt})(?:\\s*[:.]\\s*|\\s+)([A-Z0-9(“"])`, "g");
    c.labelRe = new RegExp(`(?:^|\\s)(${alt}):\\s+`, "g");
    C = c;
  }
  return C;
}

const TIMEOUT = 15000;
const timeout = () => (typeof AbortSignal !== "undefined" && AbortSignal.timeout ? AbortSignal.timeout(TIMEOUT) : undefined);
async function getJSON(u) { const r = await fetch(u, { headers: { accept: "application/json" }, signal: timeout() }); if (!r.ok) throw new Error(`${new URL(u).host} returned ${r.status}`); return r.json(); }
async function getText(u) { const r = await fetch(u, { signal: timeout() }); if (!r.ok) throw new Error(`${new URL(u).host} returned ${r.status}`); return r.text(); }

const iso = d => d.toISOString().slice(0, 10);
const daysAgo = n => { const d = new Date(); d.setUTCDate(d.getUTCDate() - n); return iso(d); };
const fromDate = days => days > 0 ? daysAgo(days) : "";   // 0 days = all time
const byCitations = win => win !== "day" && win !== "week";
// Removes markup and decodes HTML codes such as &lt; and &#8201; (a thin space, used in "69 521").
const ENT = { amp: "&", lt: "<", gt: ">", quot: '"', apos: "'", nbsp: "\u00A0", thinsp: "\u2009", ndash: "–", mdash: "—", minus: "−", plusmn: "±", times: "×", middot: "·", le: "≤", ge: "≥", micro: "µ", deg: "°" };
const decode = s => s.replace(/&(#x[0-9a-f]+|#\d+|[a-z]+);/gi, (m, e) => e[0] === "#" ? String.fromCodePoint(parseInt(e[1] === "x" || e[1] === "X" ? e.slice(2) : e.slice(1), e[1] === "x" || e[1] === "X" ? 16 : 10)) : ENT[e.toLowerCase()] ?? m);
const clean = s => decode(String(s || "").replace(/<[^>]+>/g, " ")).replace(/\s+/g, " ").trim();
// Structured abstracts: headings (HTML from Europe PMC, or plain "BACKGROUND …") become "Background: ", so the
// page shows labelled paragraphs and the summary line comes from the Results, not the background.
const HEAD_TAG = /<(h[1-6]|b|strong|sec-title|title)[^>]*>\s*([^<]{1,60}?)\s*[:.]?\s*<\/(?:h[1-6]|b|strong|sec-title|title)>/gi;
const OTHER_SENT = /\b(funded by|clinicaltrials\.gov|NCT\d{6,}|ISRCTN|PROSPERO|copyright|©)/i;
export function cleanAbstract(raw) {
  let s = String(raw || "").replace(HEAD_TAG, (m, tag, label) => {
    const c = C && C.canon.get(label.trim().toLowerCase());
    return c ? ` ${c}: ` : /^h/i.test(tag) && label.trim().split(/\s+/).length <= 6 ? ` ${label.trim()}: ` : ` ${label.trim()} `;
  });
  s = clean(s).replace(/^(Abstract: |ABSTRACT )/, "");
  if (!C || !C.plainHead) return s;
  for (let i = 0; i < 2; i++) s = s.replace(C.plainHead, (m, pre, name, next) => {
    const mid = pre !== "" && pre.trim() === "", upper = name === name.toUpperCase(), colon = m.slice(pre.length + name.length).includes(":");
    return mid && !upper && !colon ? m : `${pre}${C.canon.get(name.toLowerCase())}: ${next}`;
  });
  return s.trim();
}
// Sentences with their part of the abstract ("" when it has no headings), headings removed.
export function abstractSentences(text) {
  const out = [], marks = C && C.labelRe ? [...String(text).matchAll(C.labelRe)] : [];
  const add = (part, seg) => { for (const s of sentences(seg)) out.push({ s, part: OTHER_SENT.test(s) ? "other" : part }); };
  if (!marks.length) { add("", text); return out; }
  add("", text.slice(0, marks[0].index));
  marks.forEach((m, i) => add(C.secOf.get(m[1].toLowerCase()) || "", text.slice(m.index + m[0].length, i + 1 < marks.length ? marks[i + 1].index : text.length)));
  return out;
}
const firstAuthor = n => !n.length ? "" : n.length === 1 ? n[0] : `${n[0]} et al.`;

/* ---------- which sources to read: empty selection = every source ---------- */
const SERVERS = { biorxiv: "bioRxiv", medrxiv: "medRxiv", arxiv: "arXiv" };
export function selection(ids) {
  const set = new Set();
  for (let id of [].concat(ids || []).join(",").split(",")) {
    id = id.trim(); if (!id) continue;
    if (C.issn.has(id.toUpperCase())) set.add(id.toUpperCase());
    else if (SERVERS[id.toLowerCase()] || id.toLowerCase() === "other") set.add(id.toLowerCase());
  }
  const has = id => set.size === 0 || set.has(id);
  return { has, all: set.size === 0, key: [...set].sort().join(",") || "all",
    journals: tier => [...C.issn.values()].filter(j => (!tier || j.tier === tier) && has(j.issn)),
    anyServer: () => has("biorxiv") || has("medrxiv") || has("arxiv") };
}

/* ---------- Europe PMC ---------- */
const EPMC = "https://www.ebi.ac.uk/europepmc/webservices/rest/search";
// Reviews, letters and the like are labelled and shown in searches and categories, not in the main tabs.
const NOT_RESEARCH = /\b(review|editorial|comment|news|letter|interview|biography|congress)\b/i;
const NEVER_SHOWN = /\b(erratum|correction|retraction|published erratum|expression of concern)\b/i;
// The main tabs show original research with a real abstract only.
// Everywhere (tabs, searches, categories): original research and reviews with a real abstract only.
const isReview = p => p.kind === "Review" && p.type !== "preprint";
const digestable = p => (!p.kind || isReview(p)) && p.abstract.length > 200;
// The three paper types the page filters by.
export const paperClass = p => p.type === "preprint" ? "preprint" : p.kind === "Review" ? "review" : "research";
const SORT_NEWEST = "P_PDATE_D desc", SORT_CITED = "CITED desc";
async function epmcSearch(query, sort, pageSize) {
  const run = (q, s) => getJSON(EPMC + "?" + new URLSearchParams({ query: q, format: "json", resultType: "core", pageSize: String(pageSize), ...(s ? { sort: s } : {}) }))
    .then(j => (j.resultList && j.resultList.result) || []);
  try { return await run(query, sort); }
  catch (e) { if (!sort) throw e; return run(query + (sort.startsWith("CITED") ? " sort_cited:y" : " sort_date:y"), ""); }   // older sort syntax
}
function preprintServer(r) {
  const pub = String((r.bookOrReportDetails && r.bookOrReportDetails.publisher) || "").toLowerCase(), doi = String(r.doi || "").toLowerCase();
  if (pub.includes("medrxiv")) return "medrxiv";
  if (pub.includes("biorxiv")) return "biorxiv";
  if (pub.includes("arxiv") || doi.startsWith("10.48550/arxiv")) return "arxiv";
  if (doi.startsWith("10.1101/")) { const m = doi.match(/^10\.1101\/\d{4}\.\d{2}\.\d{2}\.(\d+)/); return m && m[1].length === 8 ? "medrxiv" : "biorxiv"; }
  return "";
}
const SMALL = new Set(["of", "and", "the", "in", "for", "on", "&"]);
const titleCase = s => s.split(/\s+/).map((w, i) => i && SMALL.has(w.toLowerCase()) ? w.toLowerCase() : w === w.toLowerCase() ? w.charAt(0).toUpperCase() + w.slice(1) : w).join(" ");
function fromEPMC(r) {
  const types = [].concat((r.pubTypeList && r.pubTypeList.pubType) || []);
  if (types.some(t => NEVER_SHOWN.test(t))) return null;
  const kindHit = types.map(t => (String(t).match(NOT_RESEARCH) || [])[0]).find(Boolean);
  const p = { title: clean(r.title), abstract: cleanAbstract(r.abstractText), date: r.firstPublicationDate || (r.pubYear ? r.pubYear + "-01-01" : ""), doi: r.doi || "",
    authorList: String(r.authorString || "").trim().replace(/\.$/, "").split(", ").map(x => x.trim()).filter(Boolean),
    citations: r.citedByCount || 0, authors: (a => { const parts = a.split(", ").filter(Boolean); return parts.length > 1 ? parts[0] + " et al." : parts[0] || ""; })(String(r.authorString || "").trim().replace(/\.$/, "")),
    fieldHint: "", fieldDefault: "", published: "", kind: kindHit ? kindHit[0].toUpperCase() + kindHit.slice(1).toLowerCase() : "" };
  if (!p.title) return null;
  if (r.source === "PPR") {
    const srv = preprintServer(r); if (!srv) return null;
    Object.assign(p, { type: "preprint", tier: 3, source: srv, venue: SERVERS[srv], fieldDefault: srv === "medrxiv" ? "medicine" : "" });
  } else {
    const j = (r.journalInfo && r.journalInfo.journal) || {};
    const hit = [j.issn, j.essn].filter(Boolean).map(i => C.issn.get(String(i).toUpperCase())).find(Boolean);
    Object.assign(p, { type: "journal", tier: 2, source: "other", venue: titleCase(String(j.title || "Journal").replace(/\s*\(.*\)$/, "").replace(/\.$/, "")) });
    if (hit) Object.assign(p, { venue: hit.name, tier: hit.tier, fieldHint: hit.field, source: hit.issn });
  }
  if (p.doi) Object.assign(p, { id: "doi:" + p.doi.toLowerCase(), url: `https://doi.org/${p.doi}` });
  else if (r.pmid) Object.assign(p, { id: "pmid:" + r.pmid, url: `https://pubmed.ncbi.nlm.nih.gov/${r.pmid}/` });
  else Object.assign(p, { id: `epmc:${r.source}:${r.id}`, url: `https://europepmc.org/article/${r.source}/${r.id}` });
  return p;
}
const journalClause = js => "(" + js.flatMap(j => [`ISSN:"${j.issn}"`, `ESSN:"${j.issn}"`]).join(" OR ") + ")";
const dateClause = from => from ? `FIRST_PDATE:[${from} TO ${daysAgo(0)}]` : "";
const epmcQuery = (...parts) => parts.filter(Boolean).join(" AND ");
// Papers with an abstract (everywhere), and no reviews in the main tabs when the reader left them out.
const WITH_ABSTRACT = "HAS_ABSTRACT:y", NOT_REVIEW = 'NOT PUB_TYPE:"review"';
const mainConditions = reviews => reviews ? WITH_ABSTRACT : `${WITH_ABSTRACT} AND ${NOT_REVIEW}`;
async function epmcPapers(query, sort, pageSize, keep) {
  return (await epmcSearch(query, sort, pageSize)).map(fromEPMC).filter(p => p && (!keep || keep(p)));
}
const epmcJournals = (js, from, sort, extra, n) => epmcPapers(epmcQuery(journalClause(js), dateClause(from), extra), sort, n);
const epmcPreprints = (sel, from, sort, extra, n) => epmcPapers(epmcQuery("SRC:PPR", extra, dateClause(from)), sort, n, p => sel.has(p.source));

/* ---------- bioRxiv / medRxiv ---------- */
async function rxiv(server, from, maxPages) {
  const base = `https://api.biorxiv.org/details/${server}/${from}/${daysAgo(0)}`;
  const first = await getJSON(`${base}/0/json`);
  const total = Number((first.messages && first.messages[0] && first.messages[0].total) || 0), pages = Math.ceil(total / 100), n = Math.min(maxPages, pages);
  const picks = new Set();
  for (let i = 1; i < n; i++) picks.add(Math.round(i * (pages - 1) / Math.max(1, n - 1)));
  picks.delete(0);
  const rest = await Promise.allSettled([...picks].map(p => getJSON(`${base}/${p * 100}/json`)));
  const rows = (first.collection || []).concat(...rest.filter(r => r.status === "fulfilled").map(r => r.value.collection || []));
  const latest = new Map();
  for (const r of rows) { const prev = latest.get(r.doi); if (!prev || Number(r.version) > Number(prev.version)) latest.set(r.doi, r); }
  return [...latest.values()].filter(r => r.type !== "withdrawn" && r.abstract && r.abstract.length > 200).map(r => ({
    id: "doi:" + String(r.doi).toLowerCase(), title: clean(r.title), abstract: cleanAbstract(r.abstract), venue: server === "medrxiv" ? "medRxiv" : "bioRxiv",
    type: "preprint", date: r.date, doi: r.doi, url: `https://doi.org/${r.doi}`, authors: firstAuthor(String(r.authors || "").split(";").map(s => s.trim()).filter(Boolean)),
    authorList: String(r.authors || "").split(";").map(s => s.trim()).filter(Boolean),
    citations: 0, tier: 3, fieldHint: (cat => cat in C.rxivField ? C.rxivField[cat] : server === "medrxiv" ? "medicine" : "")(String(r.category || "").toLowerCase()),
    published: r.published && r.published !== "NA" ? r.published : "", source: server }));
}

/* ---------- arXiv ---------- */
const xmlText = s => String(s || "").replace(/<!\[CDATA\[([\s\S]*?)\]\]>/g, "$1").replace(/&lt;/g, "<").replace(/&gt;/g, ">").replace(/&quot;/g, '"').replace(/&apos;/g, "'").replace(/&amp;/g, "&");
const tag = (x, t) => { const m = x.match(new RegExp(`<${t}[^>]*>([\\s\\S]*?)</${t}>`)); return m ? xmlText(m[1]) : ""; };
export function parseArxiv(xml) {
  return xml.split("<entry>").slice(1).map(chunk => {
    const e = chunk.split("</entry>")[0];
    const short = tag(e, "id").trim().replace(/^https?:\/\/arxiv\.org\/abs\//, "").replace(/v\d+$/, "");
    const term = (e.match(/primary_category[^>]*term="([^"]+)"/) || [])[1] || "";
    const key = Object.keys(C.arxivField).find(k => term === k || term.startsWith(k));
    return { id: "arxiv:" + short, title: clean(tag(e, "title")), abstract: cleanAbstract(tag(e, "summary")), venue: "arXiv", type: "preprint",
      date: tag(e, "published").slice(0, 10), doi: tag(e, "arxiv:doi").trim(), url: `https://arxiv.org/abs/${short}`,
      authors: firstAuthor([...e.matchAll(/<author>\s*<name>([\s\S]*?)<\/name>/g)].map(m => xmlText(m[1]).trim())),
      authorList: [...e.matchAll(/<author>\s*<name>([\s\S]*?)<\/name>/g)].map(m => xmlText(m[1]).trim()),
      citations: 0, tier: 3, fieldHint: key ? C.arxivField[key] : "", source: "arxiv" };
  }).filter(p => p.title && p.abstract.length > 200);
}
async function arxiv(from, max) {
  const cats = "(" + C.arxivCategories.map(c => "cat:" + c).join(" OR ") + ")";
  const range = `submittedDate:[${from.replace(/-/g, "")}0000 TO ${daysAgo(0).replace(/-/g, "")}2359]`;
  return parseArxiv(await getText("https://export.arxiv.org/api/query?" + new URLSearchParams({ search_query: `${cats} AND ${range}`, sortBy: "submittedDate", sortOrder: "descending", max_results: String(max) })));
}

// Papers each time range shows (the reader can change it); the candidate pool grows with it.
export const perRange = n => Math.min(200, Math.max(5, Math.round(Number(n) || 20)));
const candidates = (win, n) => Math.min(1000, Math.max(win === "day" ? 200 : win === "week" ? 300 : 150, 2 * n));
function plan(win, sel, n = 20, reviews = true) {
  const from = fromDate(C.days[win]), short = !byCitations(win), wide = win === "week";
  const order = short ? SORT_NEWEST : SORT_CITED, per = candidates(win, n), ts = [];
  for (const [tier, name] of [[1, "Leading journals"], [2, "Specialist journals"]]) {
    const js = sel.journals(tier); if (js.length) ts.push([name, () => epmcJournals(js, from, order, mainConditions(reviews), per)]);
  }
  if (short) {   // the preprint servers' own APIs have the newest papers
    if (sel.has("biorxiv")) ts.push(["bioRxiv", () => rxiv("biorxiv", from, wide ? 6 : 3)]);
    if (sel.has("medrxiv")) ts.push(["medRxiv", () => rxiv("medrxiv", from, wide ? 3 : 2)]);
    if (sel.has("arxiv")) ts.push(["arXiv", () => arxiv(from, wide ? 300 : 150)]);
  } else if (sel.anyServer()) ts.push(["Preprint servers", () => epmcPreprints(sel, from, SORT_CITED, WITH_ABSTRACT, 300)]);
  return ts;
}

/* ---------- ranking (same rules as the desktop program) ---------- */
const ABBR = /\b(e\.g|i\.e|et al|Fig|Figs|vs|approx|ca|cf|Dr|No|Eq|Ref|Refs|Ext|Suppl)\./g;
export function sentences(t) {
  return String(t).replace(ABBR, m => m.slice(0, -1) + "∯").split(/(?<=[.!?])\s+(?=[A-Z0-9("“])/).map(s => s.replace(/∯/g, ".").trim()).filter(s => s.length > 25);
}
// A paper outside the digest's fields (e.g. physics) gets "" and is left out of the main tabs.
const guessField = p => { if (p.fieldHint) return p.fieldHint; const t = p.title + " " + p.abstract; for (const [f, re] of C.rules) if (re.test(t)) return f; return p.fieldDefault || ""; };
// The sentence stating the main result: from the Results or Conclusions of a structured abstract, never its background or methods.
export const keySentence = p => {
  const all = abstractSentences(p.abstract), cue = s => C.cueRes.some(re => re.test(s));
  if (all.some(x => x.part)) {
    const k = all.find(x => (x.part === "results" || x.part === "conclusions") && C.key.test(x.s)) || all.find(x => x.part === "results" && cue(x.s))
      || all.find(x => x.part === "results") || all.find(x => x.part === "conclusions");
    if (k) return k.s;
  }
  let ss = all.filter(x => !["other", "background", "methods"].includes(x.part)).map(x => x.s);
  if (!ss.length) ss = all.filter(x => x.part !== "other").map(x => x.s);
  return ss.find(s => C.key.test(s)) || ss[Math.min(ss.length - 1, Math.max(1, Math.floor(ss.length / 2)))] || p.abstract.slice(0, 280);
};
const venueValue = p => p.tier === 1 ? 1 : (p.tier === 2 || p.published) ? 0.7 : 0.5;
// Citations per month since publication, so a three-week-old paper is not judged against one cited for a year.
const citeRate = p => (p.citations || 0) / Math.max(1, (Date.now() - Date.parse(p.date + "T00:00:00Z")) / 864e5 / 30.44 || 1);
const attention = (p, c) => c.byCitations ? (c.maxRate ? Math.log1p(citeRate(p)) / Math.log1p(c.maxRate) : 0)
  : Math.max(0, Math.min(1, 1 - ((Date.now() - Date.parse(p.date + "T12:00:00Z")) / 864e5) / Math.max(1, c.days)));
const cueValue = p => Math.min(1, C.cueRes.filter(re => re.test(p.title + " " + p.abstract)).length / 4);
// Reviews are often the most cited, so a small penalty keeps them from crowding out new findings.
const prescore = (p, c) => Math.max(0, 0.45 * venueValue(p) + 0.30 * attention(p, c) + 0.25 * cueValue(p) - (p.abstract.length < 500 ? 0.15 : 0) - (isReview(p) ? 0.10 : 0));

export function rank(papers, c, n = 20) {
  const seen = new Set(), titles = new Set();
  const pool = papers.filter(p => digestable(p) && !(c.noReviews && isReview(p))).filter(p => { const t = p.title.toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
    if (seen.has(p.id) || titles.has(t) || C.junk.test(p.title)) return false; seen.add(p.id); titles.add(t); return true; });
  c.maxCites = Math.max(0, ...pool.map(p => p.citations || 0));
  c.maxRate = Math.max(0, ...pool.map(citeRate));
  for (const p of pool) { p.pre = prescore(p, c); p.fieldGuess = guessField(p); }
  pool.sort((a, b) => b.pre - a.pre || (a.date < b.date ? 1 : -1));
  // Limits per field and journal only matter when there are more papers than room: pick within them, then fill free places.
  const pf = new Map(), pv = new Map(), fc = Math.ceil(n / 4), vc = Math.ceil(n / 5), take = new Set();
  for (const p of pool) {
    if (take.size >= n) break;
    if (!p.fieldGuess || (pf.get(p.fieldGuess) || 0) >= fc || (pv.get(p.venue) || 0) >= vc) continue;
    pf.set(p.fieldGuess, (pf.get(p.fieldGuess) || 0) + 1); pv.set(p.venue, (pv.get(p.venue) || 0) + 1); take.add(p);
  }
  for (const p of pool) { if (take.size >= n) break; if (p.fieldGuess) take.add(p); }
  coverWithin(pool, take, n, p => p.fieldGuess);
  const out = [];
  for (const p of pool) {
    if (!take.has(p)) continue;
    out.push({ id: p.id, title: p.title, venue: p.venue, source: p.source, type: p.type, date: p.date, doi: p.doi, url: p.url, authors: p.authors,
      citations: p.citations || 0, published: p.published || "", field: p.fieldGuess, score: Math.round(p.pre * 100) / 10,
      signals: [{ label: "Venue", value: venueValue(p) }, { label: c.byCitations ? "Citations" : "Recency", value: attention(p, c) }, { label: "Result cues", value: cueValue(p) }],
      keySentence: keySentence(p), abstract: p.abstract, ...(p.kind ? { kind: p.kind } : {}) });
  }
  return out;
}

// Every field and paper type (peer reviewed or preprint) among a range's candidates gets at least one of its n
// places, so filtering a range never comes up empty while it has a match. The range still shows exactly n papers:
// a missing field and type takes the place of the lowest-ranked paper whose field and type appear more than once.
// At most a third of the places change. ranked is in score order; take is the set of chosen papers (updated).
export function coverWithin(ranked, take, n, field) {
  const key = p => field(p) + "|" + paperClass(p), have = new Map(), seen = new Set(), missing = [];
  for (const p of ranked) if (take.has(p)) have.set(key(p), (have.get(key(p)) || 0) + 1);
  for (const p of ranked) { const k = key(p); if (field(p) && !take.has(p) && !have.get(k) && !seen.has(k)) { seen.add(k); missing.push(p); } }
  for (const m of missing.slice(0, Math.max(1, Math.floor(n / 3)))) {
    if (take.size < n) { take.add(m); have.set(key(m), 1); continue; }
    const out = [...ranked].reverse().find(p => take.has(p) && (have.get(key(p)) || 0) > 1);
    if (out) { take.delete(out); have.set(key(out), have.get(key(out)) - 1); take.add(m); have.set(key(m), 1); }
  }
  return take;
}

async function runAll(tasks) {
  const settled = await Promise.allSettled(tasks.map(async ([name, fn]) => { const s = Date.now(); const ps = await fn(); return { name, ps, ms: Date.now() - s }; }));
  const sources = [], lists = [];
  settled.forEach((r, i) => r.status === "fulfilled"
    ? (sources.push({ name: r.value.name, ok: true, count: r.value.ps.length, ms: r.value.ms }), lists.push(r.value.ps))
    : sources.push({ name: tasks[i][0], ok: false, count: 0, error: String(r.reason && r.reason.message || r.reason).slice(0, 120) }));
  return { sources, lists };
}

// Each range's own fetch is kept briefly in memory, so the six tabs share work.
const POOLS = new Map(), SHARE_MS = 120000;
function poolFor(key, fetch) {
  const hit = POOLS.get(key);
  if (hit && Date.now() - hit.at < SHARE_MS) return hit.promise;
  const promise = fetch().catch(e => { POOLS.delete(key); throw e; });
  POOLS.set(key, { at: Date.now(), promise });
  return promise;
}
// A longer range includes everything its shorter ranges found: whatever This month shows is a candidate for This year too.
const upTo = win => { const ids = C.windows.map(w => w.id); return ids.slice(0, ids.indexOf(win) + 1); };
async function nested(win, own) {
  const pools = await Promise.all(upTo(win).map(own));
  return { sources: pools[pools.length - 1].sources, lists: pools.flatMap(p => p.lists) };
}
const uniqueCount = lists => new Set(lists.flat().map(p => p.id)).size;

export async function buildFeed(win, sources, n = 20, reviews = true) {
  await catalog();
  n = perRange(n);
  const t0 = Date.now(), sel = selection(sources);
  const { sources: status, lists } = await nested(win, w => poolFor(`feed:${w}:${sel.key}:${candidates(w, n)}:${reviews}`, () => runAll(plan(w, sel, n, reviews))));
  const all = lists.flat(), c = { days: C.days[win], byCitations: byCitations(win), noReviews: !reviews };
  return { window: win, label: C.labels[win], from: fromDate(C.days[win]), generatedAt: new Date().toISOString(), tookMs: Date.now() - t0,
    rankedBy: "heuristic", scanned: uniqueCount(lists), sources: status, stories: rank(all, c, n) };
}

/* ---------- keyword search (same rules as the desktop program's search.go) ---------- */
const STOP = new Set(["a", "an", "and", "or", "the", "of", "in", "on", "for", "to", "with", "by", "at", "from"]);
const RANGES = { month: 30, year: 365, "5y": 1826, all: 0 }, RANGE_LABEL = { month: "past month", year: "past year", "5y": "past 5 years", all: "any time" };
export function searchTerms(q) {
  return (String(q).match(/"[^"]+"|[^\s"]+/g) || []).slice(0, 12)
    .map(t => t.replace(/"/g, "").toLowerCase().replace(/^[.,;:!?()[\]{}]+|[.,;:!?()[\]{}]+$/g, "").trim()).filter(t => t && !STOP.has(t));
}
const esc = t => t.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
const containsTerm = (text, t) => (t.includes(" ") || t.length < 4) ? new RegExp(`\\b${esc(t)}\\b`).test(text) : text.includes(t);
async function arxivSearch(terms, from, max) {
  let query = terms.map(t => t.includes(" ") ? `all:"${t}"` : `all:${t}`).join(" AND ");
  if (from) query += ` AND submittedDate:[${from.replace(/-/g, "")}0000 TO ${daysAgo(0).replace(/-/g, "")}2359]`;
  return parseArxiv(await getText("https://export.arxiv.org/api/query?" + new URLSearchParams({ search_query: query, sortBy: "relevance", sortOrder: "descending", max_results: String(max) })));
}
function matchSentence(p, terms) {
  let best = 0, out = "";
  for (const { s, part } of abstractSentences(p.abstract)) { if (part === "other") continue; const low = s.toLowerCase(), n = terms.filter(t => containsTerm(low, t)).length; if (n > best) { best = n; out = s; } }
  return out || keySentence(p);
}
export function searchRank(lists, query, n = 40, win = null) {
  const q = Array.isArray(query) ? { terms: query, authors: [], auto: false } : query, terms = q.terms;
  const best = new Map(), titles = new Map(); let maxCites = 0, maxRate = 0;
  for (const list of lists) list.forEach((p, i) => {
    const rel = 1 - i / list.length, key = p.title.toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
    if (titles.has(key) && titles.get(key) !== p.id) { const h = best.get(titles.get(key)); if (h && rel > h.rel) h.rel = rel; return; }
    if (C.junk.test(p.title) || !digestable(p)) return;   // no editorials, letters or short items
    titles.set(key, p.id);
    const h = best.get(p.id); if (!h) best.set(p.id, { p, rel }); else if (rel > h.rel) h.rel = rel;
    maxCites = Math.max(maxCites, p.citations || 0); maxRate = Math.max(maxRate, citeRate(p));
  });
  const out = [];
  for (const { p, rel } of best.values()) {
    const title = p.title.toLowerCase(), all = (p.title + " " + p.abstract).toLowerCase();
    const inAny = terms.filter(t => containsTerm(all, t)).length;
    // Authors are checked against the paper's own author list, so a namesake in the text never counts.
    let byAuthor = "", authorRole = "", authOK = q.authors.length > 0;
    q.authors.forEach((nm, i) => { if (!authOK) return; const m = authorMatch(p, nm); if (!m) authOK = false; else if (i === 0) [byAuthor, authorRole] = m; });
    if (!q.authors.length && !inAny) continue;
    if (q.auto && !authOK && !inAny) continue;
    if (q.authors.length && !q.auto && (!authOK || (terms.length && !inAny))) continue;
    const inTitle = terms.filter(t => containsTerm(title, t)).length;
    const cites = maxCites ? Math.log1p(p.citations || 0) / Math.log1p(maxCites) : 0;
    const cover = terms.length ? inAny / terms.length : 0, titleHit = terms.length ? inTitle / terms.length : 0;
    let score = win
      ? 0.25 * rel + 0.20 * cover + 0.10 * titleHit + 0.20 * venueValue(p) + 0.25 * attention(p, { ...win, maxCites, maxRate })
      : 0.35 * rel + 0.25 * cover + 0.15 * titleHit + 0.15 * venueValue(p) + 0.10 * cites;
    if (authOK && (!terms.length || q.auto))   // papers by the author come first, newest first
      score = win ? 1 + 0.5 * attention(p, { ...win, maxCites, maxRate }) + 0.3 * venueValue(p) + 0.2 * rel
        : 1 + Math.max(0, 1 - (Date.now() - Date.parse(p.date + "T00:00:00Z")) / 864e5 / 36500);
    else if (authOK) score += 1;
    out.push({ score, s: { id: p.id, title: p.title, venue: p.venue, source: p.source, type: p.type, date: p.date, doi: p.doi, url: p.url, authors: p.authors,
      citations: p.citations || 0, published: p.published || "", field: guessField(p), score: Math.round(score * 100) / 10, signals: [],
      keySentence: matchSentence(p, terms), abstract: p.abstract, ...(byAuthor ? { byAuthor, authorRole } : {}), ...(p.kind ? { kind: p.kind } : {}) } });
  }
  out.sort((a, b) => b.score - a.score || (a.s.date < b.s.date ? 1 : -1));
  const ranked = out.map(x => x.s), take = new Set(ranked.slice(0, n));
  coverWithin(ranked, take, n, s => s.field);
  return ranked.filter(s => take.has(s));
}
const termsClause = terms => "(" + terms.map(t => `TITLE_ABS:"${t.replace(/["\\]/g, "")}"`).join(" AND ") + ")";
function searchTasks(q, terms, from, sel) {
  const tc = termsClause(terms), dc = dateClause(from), ts = [], js = sel.journals(0);
  if (js.length) ts.push(["Digest journals", () => epmcPapers(epmcQuery(tc, journalClause(js), dc, WITH_ABSTRACT), "", 60)]);
  if (sel.has("other")) ts.push(["Other journals (PubMed)", () => epmcPapers(epmcQuery(tc, "SRC:MED", dc, WITH_ABSTRACT), "", 60)]);
  if (sel.anyServer()) ts.push(["Preprint servers", () => epmcPreprints(sel, from, "", `${tc} AND ${WITH_ABSTRACT}`, 100)]);
  if (sel.has("arxiv")) ts.push(["arXiv", () => arxivSearch(terms, from, 40)]);
  return ts;
}
// A reader-defined category: papers matching its keywords within one time window, pulled straight from the sources.
/* ---------- author search (same rules as the desktop program's query.go) ---------- */
const PARTICLES = new Set(["van", "von", "de", "der", "den", "da", "di", "del", "della", "du", "la", "le", "dos", "das", "ter", "ten", "bin", "al", "el"]);
const INITIALS = /^(\p{Lu}\.?-?){1,4}$/u, PROPER = /^\p{Lu}[\p{Ll}'’]+(-\p{Lu}?[\p{Ll}'’]+)*$/u;
export function looksLikeName(s) {
  if (/["0-9:()]/.test(s)) return false;
  const toks = s.replace(/,/g, " ").split(/\s+/).filter(Boolean);
  if (toks.length < 2 || toks.length > 4) return false;
  let words = 0;
  for (const t of toks) { if (PROPER.test(t)) words++; else if (!INITIALS.test(t) && !PARTICLES.has(t)) return false; }
  return words >= 1 && words <= 3;
}
const initialsOf = toks => toks.flatMap(t => t.split(/[.-]/).filter(Boolean)).map(part => INITIALS.test(part) && part.toUpperCase() === part ? part : part[0].toUpperCase()).join("");
export function parseName(s) {
  s = String(s || "").trim().replace(/^["']|["']$/g, "");
  if (!s || s.length > 80) return null;
  const i = s.indexOf(",");
  if (i > 0) return { surname: s.slice(0, i).trim(), initials: initialsOf(s.slice(i + 1).trim().split(/\s+/).filter(Boolean)), display: s };
  const toks = s.split(/\s+/), lastTok = toks[toks.length - 1], last = lastTok.replace(/\./g, "");
  if (toks.length >= 2 && INITIALS.test(lastTok) && last.toUpperCase() === last) return { surname: toks.slice(0, -1).join(" "), initials: last, display: s };
  let cut = toks.length - 1; while (cut > 0 && PARTICLES.has(toks[cut - 1].toLowerCase())) cut--;
  return { surname: toks.slice(cut).join(" "), initials: initialsOf(toks.slice(0, cut)), display: s };
}
export function parseQuery(raw) {
  const q = { raw: String(raw).trim(), terms: [], authors: [], auto: false };
  const rest = q.raw.replace(/\b(?:author|au|by):\s*(?:"([^"]+)"|(\S+))/gi, (_, a, b) => { const n = parseName(a || b); if (n && q.authors.length < 4) q.authors.push(n); return " "; });
  q.terms = searchTerms(rest);
  if (!q.authors.length && looksLikeName(q.raw)) { const n = parseName(q.raw); if (n) { q.authors = [n]; q.auto = true; } }
  q.mode = !q.authors.length ? "keywords" : q.auto ? "both" : "author";
  return q;
}
const normName = s => String(s).normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase().replace(/ß/g, "ss").replace(/’/g, "'").replace(/-/g, " ").split(/\s+/).filter(Boolean).join(" ");
// One set of initials is the other's, in order, with letters missing: "J" matches "JA" and "FJ" (someone known by a
// middle name), "JB" does not match "JA".
export const initialsCompatible = (a, b) => {
  a = a.toUpperCase(); b = b.toUpperCase(); if (!a || !b) return true;
  if (a.length < b.length) [a, b] = [b, a];
  let i = 0; for (const ch of a) if (i < b.length && b[i] === ch) i++;
  return i === b.length;
};
function authorMatch(p, n) {
  const list = p.authorList || [];
  for (let i = 0; i < list.length; i++) {
    const a = parseName(list[i]);
    if (!a || normName(a.surname) !== normName(n.surname) || !initialsCompatible(a.initials, n.initials)) continue;
    const total = list.length;
    return [list[i], total === 1 ? "sole author" : i === 0 ? `first of ${total} authors` : i === total - 1 ? `last of ${total} authors` : `author ${i + 1} of ${total}`];
  }
  return null;
}
const authorClause = ns => "(" + ns.map(n => { const s = n.surname.replace(/["\\]/g, "");
  return "(" + (n.initials ? [`AUTH:"${s} ${n.initials}"`].concat(n.initials.length > 1 ? [`AUTH:"${s} ${n.initials[0]}"`] : []) : [`AUTH:"${s}"`]).join(" OR ") + ")"; }).join(" AND ") + ")";
async function arxivAuthorSearch(ns, terms, from, max) {
  const parts = ns.flatMap(n => normName(n.surname).split(" ").filter(w => !PARTICLES.has(w)).map(w => "au:" + w))
    .concat(terms.map(t => t.includes(" ") ? `all:"${t}"` : `all:${t}`));
  let query = parts.join(" AND ");
  if (from) query += ` AND submittedDate:[${from.replace(/-/g, "")}0000 TO ${daysAgo(0).replace(/-/g, "")}2359]`;
  return parseArxiv(await getText("https://export.arxiv.org/api/query?" + new URLSearchParams({ search_query: query, sortBy: "submittedDate", sortOrder: "descending", max_results: String(max) })));
}
function queryTasks(q, from, sel) {
  const ts = [];
  if (q.terms.length && (!q.authors.length || q.auto)) ts.push(...searchTasks(q.raw, q.terms, from, sel));
  if (!q.authors.length) return ts;
  const ac = authorClause(q.authors), dc = dateClause(from), useTerms = !q.auto && q.terms.length, tc = useTerms ? termsClause(q.terms) : "", who = q.authors[0].display, js = sel.journals(0);
  if (js.length) ts.push([`Digest journals · by ${who}`, () => epmcPapers(epmcQuery(ac, tc, journalClause(js), dc, WITH_ABSTRACT), SORT_NEWEST, 100)]);
  // Indexes write initials differently ("Doudna J", "Doudna JA"): also search the digest's journals by surname
  // alone; every result is checked against the paper's own author list.
  const sc = "(" + q.authors.map(n => `AUTH:"${n.surname.replace(/["\\]/g, "")}"`).join(" AND ") + ")";
  if (js.length && sc !== ac) ts.push([`Digest journals · surname ${who}`, () => epmcPapers(epmcQuery(sc, tc, journalClause(js), dc, WITH_ABSTRACT), SORT_NEWEST, 100)]);
  if (sel.has("other")) ts.push([`Other journals · by ${who}`, () => epmcPapers(epmcQuery(ac, tc, "SRC:MED", dc, WITH_ABSTRACT), SORT_NEWEST, 100)]);
  if (sel.anyServer()) ts.push([`Preprint servers · by ${who}`, () => epmcPreprints(sel, from, SORT_NEWEST, [ac, tc, WITH_ABSTRACT].filter(Boolean).join(" AND "), 60)]);
  if (sel.has("arxiv")) ts.push([`arXiv · by ${who}`, () => arxivAuthorSearch(q.authors, useTerms ? q.terms : [], from, 40)]);
  return ts;
}

// A reader-defined category: papers matching its keywords or author within one time window, pulled straight from the sources.
export async function topic(raw, win, sources, n = 20, reviews = true) {
  n = perRange(n);
  await catalog();
  const q = parseQuery(raw);
  if (!q.terms.length && !q.authors.length) throw new Error("A category needs keywords or an author.");
  const t0 = Date.now(), days = C.days[win], from = fromDate(days), sel = selection(sources);
  const { sources: status, lists } = await nested(win, w => poolFor(`topic:${w}:${sel.key}:${q.raw.toLowerCase()}`, () => runAll(queryTasks(q, fromDate(C.days[w]), sel))));
  return { window: win, label: C.labels[win], from, query: raw, mode: q.mode, authors: q.authors.map(n => n.display), generatedAt: new Date().toISOString(),
    tookMs: Date.now() - t0, rankedBy: "category", scanned: uniqueCount(lists), sources: status, stories: searchRank(reviews ? lists : lists.map(l => l.filter(p => !isReview(p))), q, n, { days, byCitations: byCitations(win) }) };
}
export async function search(raw, range = "year", sources) {
  await catalog();
  if (!(range in RANGES)) range = "year";
  const q = parseQuery(raw);
  if (!q.terms.length && !q.authors.length) throw new Error("Enter keywords or an author.");
  const t0 = Date.now(), from = RANGES[range] ? daysAgo(RANGES[range]) : "";
  const { sources: status, lists } = await runAll(queryTasks(q, from, selection(sources)));
  return { window: "search", label: "Search · " + RANGE_LABEL[range], from, query: raw, mode: q.mode, authors: q.authors.map(n => n.display), generatedAt: new Date().toISOString(),
    tookMs: Date.now() - t0, rankedBy: "relevance", scanned: uniqueCount(lists), sources: status, stories: searchRank(lists, q) };
}

export const _clearPools = () => POOLS.clear();   // tests only
