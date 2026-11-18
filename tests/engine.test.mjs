// Checks the on-device (mobile) engine against fake Europe PMC, bioRxiv/medRxiv and arXiv responses.
import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const catalog = readFileSync(new URL("../web/catalog.json", import.meta.url), "utf8");
const today = new Date().toISOString().slice(0, 10);
const abs = t => `${t} remains poorly understood despite decades of work. We combined large datasets with new experiments across 40 samples. Here we show that ${t} is controlled by a previously unknown mechanism that explains most of the observed variation. The effect was reproduced in two independent cohorts. These results open new routes for diagnosis.`;
const rec = (src, doi, title, journal, issn, essn, publisher, cites, ...types) => ({ id: doi, source: src, doi, title, authorString: "Researcher A, Colleague B.",
  abstractText: "<h4>Background</h4>" + abs(title.toLowerCase()), firstPublicationDate: today, citedByCount: cites, pubTypeList: { pubType: ["research-article", ...types] },
  ...(journal ? { journalInfo: { journal: { title: journal, issn, essn } } } : {}), ...(publisher ? { bookOrReportDetails: { publisher } } : {}) });
const calls = [];
let rejectSort = false;
globalThis.fetch = async u => {
  u = String(u); const ok = o => new Response(typeof o === "string" ? o : JSON.stringify(o), { status: 200 });
  if (u === "catalog.json") return ok(catalog);
  if (u.includes("europepmc")) {
    const p = new URL(u).searchParams, q = p.get("query"); calls.push(q + " | sort=" + (p.get("sort") || ""));
    if (rejectSort && p.get("sort")) return new Response("", { status: 400 });
    let res = [];
    if (q.includes("SRC:PPR")) res = [rec("PPR", "10.1101/2026.10.22.760932", "Single-cell proteomics model for cells", "", "", "", "bioRxiv", 3),
      rec("PPR", "10.1101/2026.10.14.26365590", "Measles outbreaks and county vaccination coverage nationwide", "", "", "", "", 1),
      rec("PPR", "10.21203/rs.3.rs-1", "Research Square genome cells", "", "", "", "Research Square", 9)];
    else if (q.includes('ISSN:"0028-0836"')) res = [rec("MED", "10.1000/t.1", "Neuronal genome surveillance in the human brain", "Nature", "0028-0836", "1476-4687", "", 40),
      rec("MED", "10.1000/t.3", "Author Correction: x", "Nature", "0028-0836", "", "", 0, "Published Erratum"), rec("MED", "10.1000/t.5", "A review of cells", "Nature", "0028-0836", "", "", 99, "Review")];
    else if (q.includes('ESSN:"1474-760X"')) res = [rec("MED", "10.1000/t.4", "A toolkit for spatial transcriptomics", "Genome biology", "1474-7596", "1474-760X", "", 5)];
    else if (q.includes("SRC:MED")) res = [rec("MED", "10.1000/t.9", "Genome surveillance of neurons in a cohort", "journal of unknown studies", "9999-9999", "", "", 2)];
    // Like Europe PMC: honour the abstract and review conditions, so a query that leaves reviews out is caught.
    if (q.includes('NOT PUB_TYPE:"review"')) res = res.filter(r => !r.pubTypeList.pubType.some(t => /review/i.test(t)));
    if (q.includes("HAS_ABSTRACT:y")) res = res.filter(r => r.abstractText);
    const terms = [...q.matchAll(/TITLE_ABS:"([^"]+)"/g)].map(m => m[1]);
    if (terms.length) res = res.filter(r => terms.every(t => (r.title + " " + r.abstractText).toLowerCase().includes(t)));
    return ok({ resultList: { result: res } });
  }
  if (u.includes("api.biorxiv.org")) return ok({ messages: [{ total: 1 }], collection: [{ doi: "10.1101/x.1", title: "Heart atlas", authors: "Lee, J.", date: today, version: "1", type: "new results", category: "genomics", abstract: abs("heart"), published: "NA" }] });
  if (u.includes("export.arxiv.org")) return ok(`<feed><entry><id>http://arxiv.org/abs/2610.1v1</id><published>${today}T00:00:00Z</published><title>Q &amp; A</title><summary>${abs("enhancer genome")}</summary><author><name>C M</name></author><arxiv:primary_category term="q-bio.GN"/></entry></feed>`);
  return new Response("", { status: 404 });
};
const engine = await import("../web/engine.js");
const reset = () => { calls.length = 0; rejectSort = false; engine._clearPools && engine._clearPools(); };

test("today: journals from Europe PMC plus the preprint servers' own APIs", async () => {
  reset();
  const f = await engine.buildFeed("day");
  assert.deepEqual(f.sources.map(s => s.name), ["Leading journals", "Specialist journals", "bioRxiv", "medRxiv", "arXiv"]);
  assert.ok(f.sources.every(s => s.ok));
  assert.ok(!f.stories.some(s => /Correction/.test(s.title)));
  assert.ok(f.stories.filter(s => /review/.test(s.title)).every(s => s.kind === "Review"), "reviews appear labelled");;
  assert.ok(f.stories.some(s => s.id === "arxiv:2610.1" && s.source === "arxiv" && s.field === "omics"));
  assert.ok(calls.some(c => c.includes("sort=P_PDATE_D desc")));
});
test("longer ranges rank by citations and include Europe PMC preprints; all time has no date limit", async () => {
  reset();
  const y = await engine.buildFeed("year");
  assert.equal(y.stories[0].signals[1].label, "Citations");
  const by = Object.fromEntries(y.stories.map(s => [s.source, s]));
  assert.equal(by["1474-760X"].venue, "Genome Biology");
  assert.equal(by.medrxiv.venue, "medRxiv"); assert.equal(by.medrxiv.field, "publichealth");
  assert.ok(!y.stories.some(s => /Research Square/.test(s.title)));
  assert.ok(!by["0028-0836"].abstract.includes("<h4>"));
  reset();
  const a = await engine.buildFeed("all");
  assert.equal(a.from, ""); assert.ok(a.stories.length > 0);
  assert.ok(calls.some(c => c.includes('ISSN:"0028-0836"') && !c.includes("FIRST_PDATE")), "all time's own query has no date limit");
});
test("falls back to the older sort syntax", async () => {
  reset(); rejectSort = true;
  const m = await engine.buildFeed("month");
  assert.ok(m.stories.length > 0); assert.ok(calls.some(c => c.includes("sort_cited:y")));
});
test("source selection limits what is read", async () => {
  reset();
  const f = await engine.buildFeed("week", ["1474-760x", "arxiv"]);
  assert.deepEqual(f.sources.map(s => s.name), ["Specialist journals", "arXiv"]);
  assert.ok(f.stories.every(s => s.source === "1474-760X" || s.source === "arxiv"));
  const s = await engine.search("proteomics", "all", ["biorxiv"]);
  assert.equal(s.stories.length, 1); assert.equal(s.stories[0].source, "biorxiv");
  const o = await engine.search("genome surveillance", "year", ["other"]);
  assert.equal(o.stories[0].venue, "Journal of Unknown Studies");
});
test("keyword search and categories", async () => {
  reset();
  assert.deepEqual(engine.searchTerms('the "Spatial Transcriptomics" of CRISPR,'), ["spatial transcriptomics", "crispr"]);
  const f = await engine.search("genome surveillance", "year");
  assert.match(f.stories[0].title, /genome surveillance/i);
  assert.ok(calls.some(c => c.includes('TITLE_ABS:"genome"') && c.includes('TITLE_ABS:"surveillance"')));
  const t = await engine.topic("genome surveillance", "5y");
  assert.equal(t.window, "5y"); assert.ok(t.stories.length > 0);
  await assert.rejects(engine.search("  the  "), /keywords/);
});

test("longer ranges contain everything shorter ranges show", async () => {
  reset();
  let prev = new Set();
  for (const w of ["day", "week", "month", "year", "5y", "all"]) {
    const cur = new Set((await engine.buildFeed(w)).stories.map(s => s.id));
    for (const id of prev) assert.ok(cur.has(id), `${w} is missing ${id}`);
    prev = cur;
  }
  const m = await engine.topic("genome", "month"), y = await engine.topic("genome", "year");
  for (const s of m.stories) assert.ok(y.stories.some(x => x.id === s.id));
});
test("author names and author search", async () => {
  reset();
  for (const [inp, sur, ini] of [["Jennifer A. Doudna", "Doudna", "JA"], ["Doudna JA", "Doudna", "JA"], ["Doudna, Jennifer", "Doudna", "J"], ["Johannes van der Waals", "van der Waals", "J"]]) {
    const n = engine.parseName(inp); assert.equal(n.surname, sur, inp); assert.equal(n.initials, ini, inp);
  }
  assert.ok(engine.looksLikeName("Feng Zhang")); assert.ok(!engine.looksLikeName("gene therapy")); assert.ok(!engine.looksLikeName("Doudna"));
  const q = engine.parseQuery('CRISPR author:"Jennifer Doudna"');
  assert.deepEqual(q.terms, ["crispr"]); assert.equal(q.mode, "author");
  assert.equal(engine.parseQuery("Feng Zhang").mode, "both");

  const f = await engine.search('author:"Colleague B"', "year");
  assert.equal(f.mode, "author"); assert.ok(f.stories.length > 0);
  assert.ok(f.stories.every(s => s.byAuthor === "Colleague B" && s.authorRole === "last of 2 authors"));
  assert.ok(calls.some(c => c.includes('AUTH:"Colleague B"') && c.includes("sort=P_PDATE_D desc")));
  assert.equal((await engine.search("author:Nobody", "year")).stories.length, 0);
  const c = await engine.topic('author:"Colleague B"', "month");
  assert.equal(c.mode, "author"); assert.ok(c.stories.length > 0);
});

test("filters never empty a longer range: each field keeps a few of each paper type", async () => {
  const E = await import("../web/engine.js");
  const ps = [];
  for (let i = 0; i < 60; i++) ps.push({ id: "m" + i, title: "Landmark trial " + i, abstract: "We show therapy works. ".repeat(40), venue: "Journal " + (i % 9),
    type: "journal", date: "2012-01-01", citations: 5000 + i, tier: 1, fieldHint: ["medicine", "cellmol", "omics", "publichealth"][i % 4] });
  ps.push({ id: "new", title: "A new microfluidic sensor", abstract: "We show a sensor. ".repeat(40), venue: "bioRxiv", type: "preprint", date: new Date().toISOString().slice(0, 10), citations: 0, tier: 3, fieldHint: "bioeng" });
  const got = E.rank(ps, { days: 0, byCitations: true }, 30);
  assert.equal(got.length, 30, "exactly the number asked for");
  assert.ok(got.find(s => s.id === "new"), "the only bioengineering preprint keeps a place");
});

test("structured abstracts: headings become labels and the summary line comes from Results", async () => {
  const E = await import("../web/engine.js");
  await E.buildFeed("day", []).catch(() => {});   // loads the catalog
  const a = E.cleanAbstract("<h4>Background</h4>Sepsis kills 20% of patients. <h4>Methods</h4>We enrolled 1,204 adults in 12 hospitals. <h4>Results</h4>Mortality was 18.2% with early antibiotics and 24.6% with usual care. <h4>Conclusions</h4>Early antibiotics saved lives.");
  assert.equal(a, "Background: Sepsis kills 20% of patients. Methods: We enrolled 1,204 adults in 12 hospitals. Results: Mortality was 18.2% with early antibiotics and 24.6% with usual care. Conclusions: Early antibiotics saved lives.");
  assert.equal(E.cleanAbstract("BACKGROUND Sepsis is common. METHODS We did it. RESULTS It worked well."), "Background: Sepsis is common. Methods: We did it. Results: It worked well.");
  assert.equal(E.cleanAbstract("Our Results suggest a role. The Methods We used are new."), "Our Results suggest a role. The Methods We used are new.");
  assert.equal(E.keySentence({ abstract: a }), "Mortality was 18.2% with early antibiotics and 24.6% with usual care.");
});

test("HTML codes in abstracts are decoded", async () => {
  const E = await import("../web/engine.js");
  assert.equal(E.cleanAbstract("We profiled 69&#8201;521 genes (p &lt; 0.001) &amp; more."), "We profiled 69 521 genes (p < 0.001) & more.");
});

test("authors known by a middle name: 'Jeffrey Dilworth' matches 'Dilworth FJ'", async () => {
  const E = await import("../web/engine.js");
  for (const [a, b, ok] of [["JA", "J", true], ["FJ", "J", true], ["FJ", "F", true], ["JA", "JB", false], ["FJ", "JF", false]]) assert.equal(E.initialsCompatible(a, b), ok, `${a} ~ ${b}`);
  assert.equal(E.perRange(500), 200); assert.equal(E.perRange(1), 5); assert.equal(E.perRange("50"), 50); assert.equal(E.perRange(undefined), 20);
});

test("reviews are their own paper type, and can be left out", async () => {
  const E = await import("../web/engine.js");
  const long = "We review how enhancers integrate signals in muscle stem cells. ".repeat(8);
  const rev = { id: "rev", title: "Enhancers in muscle stem cells", abstract: long, kind: "Review", venue: "Skeletal Muscle", type: "journal", date: new Date().toISOString().slice(0, 10), citations: 0, tier: 2, fieldHint: "cellmol" };
  const res = { ...rev, id: "res", title: "A new finding in muscle", abstract: "We show a new finding. ".repeat(15), kind: "" };
  const on = E.rank([rev, res], { days: 365, byCitations: true }, 30).map(s => s.id + "/" + E.paperClass(s)).sort().join(",");
  assert.equal(on, "res/research,rev/review");
  const off = E.rank([rev, res], { days: 365, byCitations: true, noReviews: true }, 30).map(s => s.id).join(",");
  assert.equal(off, "res");
});

test("searches ask Europe PMC for reviews and papers without an abstract; the main tabs ask for abstracts", async () => {
  const E = await import("../web/engine.js");
  calls.length = 0; E._clearPools && E._clearPools();
  const f = await E.buildFeed("year", [], 30, true);
  assert.ok(calls.every(q => !q.includes("PUB_TYPE")), "reviews are not excluded when included");
  assert.ok(calls.some(q => q.includes("HAS_ABSTRACT:y")), "main tabs ask for abstracts");
  assert.ok(f.stories.some(s => s.kind === "Review"), "a review reaches the main tabs");
  calls.length = 0; E._clearPools && E._clearPools();
  const off = await E.buildFeed("year", [], 30, false);
  assert.ok(calls.some(q => q.includes('NOT PUB_TYPE:"review"')) && !off.stories.some(s => s.kind === "Review"), "left out when turned off");
  calls.length = 0; E._clearPools && E._clearPools();
  const t = await E.topic("cells", "year", [], 30, true);
  assert.ok(calls.every(q => q.includes("HAS_ABSTRACT:y") && !q.includes("PUB_TYPE")), "searches ask for abstracts and don't exclude reviews in the query: " + calls.join(" / "));
  assert.ok(t.stories.some(s => s.kind === "Review"), "a search finds the review");
});
