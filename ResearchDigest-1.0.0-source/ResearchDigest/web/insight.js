// Builds a small visual summary of a paper from its own title and abstract: the headline figure
// (a comparison, a percentage, a fold change, a ratio or the sample size), the study design, what was
// studied, and a key named entity. Every number shown comes from the abstract's text; nothing is invented.

// A number, with a minus sign only where it cannot be a range dash ("−1.2 points", not "18-65").
const NUM = String.raw`((?:(?<![\w.%)\]])[−-])?(?:\d{1,3}(?:,\d{3})+|\d+(?:\.\d+)?))`;
const num = s => parseFloat(String(s).replace(/,/g, "").replace("−", "-"));
const fmt = n => (Math.abs(n) >= 1000 ? Math.round(n).toLocaleString("en-US") : String(+n.toFixed(2)));
const clip = (s, n) => (s.length > n ? s.slice(0, n - 1).trimEnd() + "…" : s);
const cap = s => s ? s[0].toUpperCase() + s.slice(1) : s;

// Abstracts use "·" or "·" as a decimal point in some journals (The Lancet), and en-dashes in ranges.
// Numbers written with spaces between thousands ("69 521", often a thin space) become "69,521" so they read
// as one number; a space before a percentage ("week 4 100%") is never taken for a thousands gap.
const GROUPED = /(?<![\d.,])(\d{1,3})((?:[ \u00A0\u2009\u202F]\d{3})+)(?!\d|[.,]\d|\s?%)/g;
const normalise = t => String(t || "").replace(/(\d)[·∙](\d)/g, "$1.$2")
  .replace(GROUPED, (m, head, rest) => head + rest.replace(/[ \u00A0\u2009\u202F]/g, ","))
  .replace(/[\u00A0\u2009\u202F\u2007]/g, " ").replace(/\s+/g, " ");

const SENT = s => s.split(/(?<=[.!?])\s+(?=[A-Z(])/).map(x => x.trim()).filter(Boolean);

/* ---------- which part of the abstract a sentence is in ---------- */
// Structured abstracts arrive with their section names ("Background: … Methods: … Results: …"). Figures are
// read from Results and Conclusions only, so a statistic from the background or the study design never
// stands in for the finding.
const SECTION_OF = [
  ["results", /^(results?|findings|methods and findings|main results|key results|outcomes)$/i],
  ["methods", /^(methods?|methodology|materials and methods|design|setting|design,? setting,? and participants|participants|patients|population|interventions?|exposures?|main outcomes? and measures?|measurements?|study design|data sources?|study selection|data extraction(?: and synthesis)?|eligibility criteria|approach)$/i],
  ["conclusions", /^(conclusions?|conclusions? and relevance|interpretation|significance|implications|discussion|summary)$/i],
  ["background", /^(background|introduction|objectives?|aims?|purpose|importance|context|rationale|motivation|aims? and objectives?|background and aims?|question)$/i],
  ["other", /^(funding|trial registration|registration|availability(?: and implementation)?|supplementary information|contact|clinical trial registration|keywords?)$/i]
];
const LABEL_RE = /(?:^|\s)([A-Z][A-Za-z ,/-]{2,48}?):\s+(?=\S)/g;
const sectionOf = label => (SECTION_OF.find(([, re]) => re.test(label.trim())) || [null])[0];
const METHOD_SENT = /\b(were (randomly )?(assigned|allocated|enrolled|recruited|included|screened|followed|analy[sz]ed)|we (enrolled|recruited|randomly assigned|randomi[sz]ed|conducted|performed|included|screened|collected|used data|analy[sz]ed data|searched)|this (phase \d|randomi[sz]ed|multicent|prospective|retrospective|cohort|cross-sectional)|was (powered|designed) to)\b/i;
const OTHER_SENT = /\b(funded by|clinicaltrials\.gov|NCT\d{6,}|ISRCTN|PROSPERO|CRD\d{6,}|copyright|©)/i;
function sections(abstract) {
  const out = [];
  const marks = [...abstract.matchAll(LABEL_RE)].filter(m => sectionOf(m[1]));
  const parts = marks.length ? [[null, abstract.slice(0, marks[0].index)], ...marks.map((m, i) => [sectionOf(m[1]), abstract.slice(m.index + m[0].length, i + 1 < marks.length ? marks[i + 1].index : abstract.length)])]
    : [[null, abstract]];
  for (const [sec, text] of parts) for (const s of SENT(text)) out.push({ s, sec });
  // Unstructured abstracts: the opening sentence is usually background, and design sentences are methods.
  if (!marks.length) out.forEach((x, i) => {
    x.sec = OTHER_SENT.test(x.s) ? "other" : i === 0 && out.length >= 3 && !CLAIM_TEST.test(x.s) ? "background" : METHOD_SENT.test(x.s) ? "methods" : null;
  });
  else out.forEach(x => { if (OTHER_SENT.test(x.s)) x.sec = "other"; });
  return out;
}
const CLAIM_TEST = /\b(here,? we|we (show|find|found|report|demonstrate|reveal|identify)|our (results|findings))\b/i;

/* ---------- what was studied ---------- */
const SUBJECTS = [
  ["Humans", /\b(patients?|participants?|adults?|children|infants?|individuals|people|volunteers|women|men|persons|subjects|donors|cohort)\b/gi],
  ["Mice", /\b(mice|mouse|murine)\b/gi],
  ["Rats", /\brats?\b/gi],
  ["Primates", /\b(macaques?|monkeys?|non-human primates?|marmosets?)\b/gi],
  ["Zebrafish", /\bzebrafish\b/gi],
  ["Flies", /\b(drosophila|fruit fl(y|ies))\b/gi],
  ["Worms", /\b(c\. elegans|caenorhabditis|nematodes?)\b/gi],
  ["Organoids", /\borganoids?\b/gi],
  ["Plants", /\b(arabidopsis|plants?|crops?|rice|maize|wheat)\b/gi],
  ["Microbes", /\b(bacteria|bacterial|e\. coli|yeast|archaea|microbes?|microbiome)\b/gi],
  ["Cells", /\b(cell lines?|in vitro|cultured cells|human cells|ipscs?|stem cells)\b/gi],
  ["Computational", /\b(simulations?|in silico|language models?|neural networks?|deep learning|machine learning|algorithms?|benchmarks?|datasets?|computational)\b/gi],
  ["Quantum hardware", /\b(qubits?|quantum processors?|trapped[- ]ions?|superconducting)\b/gi],
  ["Materials", /\b(electrolytes?|alloys?|polymers?|crystals?|thin films?|nanomaterials?|catalysts?|semiconductors?)\b/gi],
  ["Devices", /\b(devices?|chips?|sensors?|batter(y|ies)|lasers?|robots?)\b/gi],
  ["Space", /\b(galax(y|ies)|black holes?|exoplanets?|stars?|telescope)\b/gi]
];
function subject(text) {
  let best = null;
  SUBJECTS.forEach(([label, re], i) => {
    const n = (text.match(re) || []).length;
    if (n && (!best || n > best.n)) best = { label, n, i };
  });
  return best ? best.label : null;
}

/* ---------- how it was studied ---------- */
const DESIGNS = [
  [/\bphase (3|iii)\b/i, "Phase 3 trial", "trial"], [/\bphase (2|ii)\b/i, "Phase 2 trial", "trial"], [/\bphase (1|i)\b(?![a-z])/i, "Phase 1 trial", "trial"],
  [/\brandomi[sz]ed\b/i, "Randomized trial", "trial"], [/\bmeta-analys[ie]s\b/i, "Meta-analysis", "stack"],
  [/\b(systematic review)\b/i, "Systematic review", "stack"], [/\bhuman challenge\b/i, "Challenge study", "trial"],
  [/\b(prospective|retrospective) (cohort|study)\b|\bcohort study\b|\bregistry\b|\bpopulation-based\b/i, "Cohort study", "people"],
  [/\bcase[- ]control\b/i, "Case-control study", "people"], [/\bcross-sectional\b/i, "Cross-sectional study", "people"],
  [/\bgenome-wide association|\bgwas\b/i, "GWAS", "dna"], [/\bmendelian randomi[sz]ation\b/i, "Mendelian randomization", "dna"],
  [/\b(crispr|genetic|genome-wide) screens?\b/i, "Genetic screen", "dna"],
  [/\b(single-cell|single-nucleus|spatial)\b.*\b(atlas|map|transcriptom\w*|profil\w*)/i, "Single-cell atlas", "grid"],
  [/\batlas\b/i, "Atlas", "grid"], [/\bcryo-?em|cryo-electron|crystal structure|x-ray crystallograph/i, "Structure", "cube"],
  [/\b(we (present|introduce|develop|propose|describe)|toolkit|framework|pipeline|software package|open-source)\b/i, "New method", "tool"],
  [/\bbenchmark(s|ing)?\b/i, "Benchmark", "bars"], [/\b(dataset|database|resource|corpus)\b/i, "Data resource", "grid"],
  [/\b(simulat\w+|numerical model|theoretical|we derive|analytical model)\b/i, "Theory & simulation", "wave"],
  [/\b(we (demonstrate|realize|fabricate)|experimental(ly)? demonstrat)/i, "Demonstration", "spark"],
  [/\bmechanism\b/i, "Mechanism", "gear"], [/\bobserv(ation|ed)\b/i, "Observation", "eye"],
  [/\b(here,? we (show|report|find|identify|discover|reveal)|we (show|find|found|report|identify|discover|reveal) that)\b/i, "New finding", "spark"]
];
function design(text, type, kind) {
  for (const [re, label, icon] of DESIGNS) if (re.test(text)) return { label, icon };
  if (kind === "Review") return { label: "Review", icon: "doc" };
  return { label: type === "preprint" ? "Preprint study" : "Research article", icon: "doc" };
}

/* ---------- how big ---------- */
const UNITS = "patients|participants|adults|children|infants|individuals|people|volunteers|women|men|donors|mice|rats|macaques|cells|neurons|nuclei|samples|genomes|species|cohorts|hospitals|countries|sites|families|twins|tumou?rs|images|proteins|genes|variants|spectra|structures|languages|tasks|benchmarks|qubits|papers|studies|trials|records|admissions|encounters|visits|users|households|pregnancies|births|deaths|cases|events|reads";
const COUNT_RE = new RegExp(String.raw`\b(\d{1,3}(?:,\d{3})+|\d+(?:\.\d+)?\s?(?:million|billion)|\d{2,})\s+(?:[A-Za-z-]+\s+){0,2}?(${UNITS})\b`, "gi");
function sampleSize(text) {
  let best = null;
  if (typeof text !== "string") text = text.join(" ");
  for (const m of text.matchAll(COUNT_RE)) {
    const raw = m[1], big = /million|billion/i.test(raw);
    let n = num(raw); if (/million/i.test(raw)) n *= 1e6; if (/billion/i.test(raw)) n *= 1e9;
    if (n < 3 || (!big && !/,/.test(raw) && n >= 1900 && n <= 2100)) continue;          // years
    if (/\b(worldwide|globally|annually|each year|per year|every year|a year|estimated|affects?|affecting)\b/i.test(text.slice(m.index, m.index + m[0].length + 30))) continue;
    if (/^\d+\.\d+$/.test(raw) && !big) continue;                                        // "2.5 cells" is not a count
    if (!best || n > best.n) best = { n, value: raw.replace(/\s+/g, " "), unit: m[2].toLowerCase() };
  }
  return best;
}

/* ---------- the headline figure ---------- */
// Units a comparison may be in; never part of a longer word ("12 genes" is not 12 g).
const UNIT = String.raw`((?:%|percentage points?|months?|years?|weeks?|days?|hours?|minutes?|seconds?|mg\/dl|mg\/l|mmol\/l|nmol\/l|μmol\/l|µmol\/l|ng\/ml|pg\/ml|iu\/ml|u\/l|mm ?hg|kg\/m2|kg\/m²|kg|mg|μg|µg|ng|ml|mm|cm|μm|µm|nm|g|l|bpm|°c|k|ms|points?)(?![A-Za-z]))`;
const ARM = String.raw`(?:\s+(?:with|in|for|among|on|using|under|after|receiving|from)\s+(?:the\s+)?([^,;.()]{2,42}?))?`;
const ARM2 = String.raw`(?:\s+(?:with|in|for|among|on|using|under|after|receiving|from)\s+(?:the\s+)?([^,;.()]{2,42}))?`;
const JOIN = String.raw`\s*,?\s*(?:versus|vs\.?|compared with|compared to|as compared with|and|to)\s+`;
const COMPARE_RE = new RegExp(String.raw`${NUM}\s?${UNIT}${ARM}${JOIN}${NUM}\s?${UNIT}?${ARM2}(?=[\s,;.)]|$)`, "gi");
const FROMTO_RE = new RegExp(String.raw`from\s+${NUM}\s?${UNIT}?\s+(?:at baseline\s+)?to\s+${NUM}\s?${UNIT}`, "gi");
const FOLD_RE = new RegExp(String.raw`${NUM}\s?(?:-fold|-times|×|x\b|fold)\s*([a-z][^,;.()]{0,40})?`, "gi");
// Ratio names in words match in any case; the abbreviations only in capitals, so "or" and "hr" (hours) never count.
const RATIO_RE = /\b((?:[Hh]azard|[Oo]dds|[Rr]elative risk|[Rr]isk|[Ii]ncidence rate|[Rr]ate) ratios?|[Rr]elative risk|aHR|aOR|HR|OR|RR|IRR)\b[^0-9]{0,24}(\d+\.\d+)/g;
const AUC_RE = /\b(AUC|AUROC|area under the (?:receiver operating characteristic )?curve|C-statistic)[^0-9]{0,24}(0\.\d+)/gi;
const PCT_RE = new RegExp(String.raw`${NUM}\s?%`, "g");
const RESULT_WORD = /\b(reduc|lower|decreas|increas|improv|higher|fewer|more|less|accura|sensitiv|specific|success|response|survival|efficien|risk|rate|yield|recall|precision|correct|outperform|fell|rose|declin|gain|loss)/i;
const CI_NEAR = /(\d+\s?%\s*(confidence|CI|credible))|\b(CI|IQR|SD|SE|range)\b|interquartile|standard deviation/i;
// Percentages that describe the study rather than its result: power, thresholds, who took part.
const NOT_RESULT_PCT = /\b(power|significance level|alpha|threshold|cut-?off|confidence|credible|CI|IQR|quantile|percentile)\b/i;
const DEMOG_AFTER = /^\s*(?:were\s+|was\s+)?(?:women|men|female|male|girls|boys|white|black|asian|hispanic|latin\w*|smokers|aged|of (?:the )?(?:participants|patients|respondents|cohort) were)\b/i;
const RANGE_BEFORE = /\b(rang\w*|between|aged|ages?|age range|varied|spanning|IQR|interquartile)\s*(?:from\s*)?$/i;
const STOPTAIL = /\s+(was|were|is|are|of|by|at|to|reached|achieved|showed|had|with|in|for|from|that|the|a|an|than|and|or|as|on|nearly|about|approximately|roughly|only|just|over|under|up)$/i;
const STOPHEAD = /^(the|a|an|and|but|while|whereas|with|in|of|overall,?|here,?|we found that|we show that|compared)\s+/i;

function phraseBefore(text, idx, words = 6) {
  const before = text.slice(Math.max(0, idx - 120), idx).split(/[,;:()]|\b(?:and|but|while|whereas)\b/).pop() || "";
  let w = before.trim().split(/\s+/).slice(-words).join(" ");
  for (let i = 0; i < 4; i++) w = w.replace(STOPTAIL, "").replace(STOPHEAD, "");
  return w.trim();
}
function phraseAfter(text, idx, words = 6) {
  const after = text.slice(idx, idx + 120).split(/[,;:().]/)[0] || "";
  return after.trim().split(/\s+/).slice(0, words).join(" ").replace(STOPTAIL, "").trim();
}
const cleanArm = s => s ? clip(s.replace(/\s+(respectively|in total|at \w+|over \w+|after \w+)$/i, "").replace(STOPTAIL, "").trim(), 28) : "";
const unitLabel = u => !u ? "" : u === "%" ? "%" : /^mm ?hg$/i.test(u) ? "mm Hg" : /^(ml|l)$/i.test(u) ? u.replace(/l$/i, "L") : u.toLowerCase().replace(/^months?$/, "months").replace(/^years?$/, "years").replace(/^weeks?$/, "weeks").replace(/^days?$/, "days").replace(/^hours?$/, "hours");

// Sentences a headline figure may come from, best first: the key sentence, then Results, then Conclusions,
// then unlabelled sentences; within each, sentences with result words first, then in reading order.
function bestSentenceOrder(abstract, key) {
  const SEC_W = { results: 4, conclusions: 2, null: 3 };
  return sections(abstract).filter(x => !["background", "methods", "other"].includes(x.sec))
    .map((x, i) => ({ s: x.s, w: (x.s === key || (key && key.includes(x.s)) ? 10 : 0) + SEC_W[x.sec] + (RESULT_WORD.test(x.s) ? 2 : 0) - i / 100 }))
    .sort((a, b) => b.w - a.w).map(x => x.s);
}

function figure(title, abstract, key, n) {
  // The best sentence decides: its own comparison, ratio, fold change or percentage, in that order.
  for (const s of bestSentenceOrder(abstract, key)) {
    const f = compareIn(s) || ratioIn(s) || foldIn(s) || percentIn(s);
    if (f) return f;
  }
  // Otherwise, the size of the study.
  if (n) return { kind: "count", value: n.value, unit: n.unit };
  return null;
}

// Two comparable values: "21.1 months with X and 12.7 months with Y", "from 8.8% to 7.2%".
function compareIn(s) {
  for (const m of s.matchAll(FROMTO_RE)) {
    const u1 = m[2] || m[4], u2 = m[4];
    if (u1 && u2 && u1.toLowerCase() !== u2.toLowerCase()) continue;
    const a = num(m[1]), b = num(m[3]); if (a === b || CI_NEAR.test(s.slice(Math.max(0, m.index - 20), m.index + m[0].length + 10))) continue;
    if (RANGE_BEFORE.test(s.slice(Math.max(0, m.index - 24), m.index)) || /^\s*(of age|old|in age)\b/i.test(s.slice(m.index + m[0].length))) continue;
    return { kind: "compare", unit: unitLabel(u2), caption: cap(phraseBefore(s, m.index)) || "Change", a: { value: a, label: "Before" }, b: { value: b, label: "After" } };
  }
  for (const m of s.matchAll(COMPARE_RE)) {
    const u1 = m[2], u2 = m[5] || m[2];
    if (u1.toLowerCase() !== u2.toLowerCase()) continue;
    const a = num(m[1]), b = num(m[4]);
    if (a === b || CI_NEAR.test(s.slice(Math.max(0, m.index - 24), m.index + m[0].length + 6))) continue;
    if (RANGE_BEFORE.test(s.slice(Math.max(0, m.index - 24), m.index)) || /^\s*(of age|old)\b/i.test(s.slice(m.index + m[0].length))) continue;
    if (/^(to|and)$/i.test((m[0].match(/\b(versus|vs\.?|compared with|compared to|and|to)\b/i) || [])[0] || "") && !m[3] && !m[6] && !RESULT_WORD.test(s)) continue; // plain ranges
    if (u1 === "%" && (a > 100 || b > 100)) continue;
    return { kind: "compare", unit: unitLabel(u1), caption: cap(phraseBefore(s, m.index)) || "Comparison",
      a: { value: a, label: cleanArm(m[3]) }, b: { value: b, label: cleanArm(m[6]) } };
  }
  return null;
}

// Ratios with a clear name (hazard ratio, odds ratio) and model performance (AUC).
function ratioIn(s) {
  for (const m of s.matchAll(RATIO_RE)) {
    const name = /hazard|HR/i.test(m[1]) ? "Hazard ratio" : /odds|OR/.test(m[1]) || /odds/i.test(m[1]) ? "Odds ratio" : /incidence|IRR|rate ratio/i.test(m[1]) ? "Rate ratio" : "Relative risk";
    const v = num(m[2]); if (v <= 0 || v > 20) continue;
    const after = phraseAfter(s, m.index + m[0].length, 6).replace(/^[\s;,]*(95% CI.*)?/, "");
    return { kind: "ratio", name, value: v, caption: cap(phraseBefore(s, m.index)) || after || name };
  }
  for (const m of s.matchAll(AUC_RE)) {
    const v = num(m[2]); if (v < 0.5 || v > 1) continue;
    return { kind: "ratio", name: "AUC", value: v, caption: cap(phraseBefore(s, m.index)) || "Prediction accuracy (1.0 is perfect)" };
  }
  return null;
}

// Fold changes: "fell 4.5-fold", "10-fold more sensitive". Not scientific notation (5 × 10⁵) or dosing (2 times daily).
function foldIn(s) {
  for (const m of s.matchAll(FOLD_RE)) {
    const v = num(m[1]); if (v < 1.2 || v > 1e6) continue;
    const rest = s.slice(m.index + m[0].length - (m[2] ? m[2].length : 0));
    if (/x\b|×/.test(m[0]) && !/fold|times/.test(m[0]) && (!RESULT_WORD.test(s) || /^\s*10\b|^\s*10[⁰¹²³⁴⁵⁶⁷⁸⁹^]/.test(rest))) continue;
    if (/times/.test(m[0]) && /^\s*(daily|a day|per|weekly|a week|each|every)\b/i.test(rest)) continue;
    const tail = m[2] ? m[2].replace(STOPTAIL, "").trim() : "";
    const head = phraseBefore(s, m.index);
    return { kind: "fold", value: v, caption: cap(clip([head, tail].filter(Boolean).join(" ") || "Fold change", 64)) };
  }
  return null;
}

// A single percentage tied to a result word ("reduced LDL cholesterol by 62%", "94% accuracy").
function percentIn(s) {
  if (!RESULT_WORD.test(s)) return null;
  for (const m of s.matchAll(PCT_RE)) {
    const v = num(m[1]); if (v <= 0 || v > 100) continue;
    const around = s.slice(Math.max(0, m.index - 24), m.index + m[0].length + 18);
    if (NOT_RESULT_PCT.test(around) || DEMOG_AFTER.test(s.slice(m.index + m[0].length))) continue;
    if (RANGE_BEFORE.test(s.slice(Math.max(0, m.index - 24), m.index))) continue;
    if (/^\s*[–-]\s*\d/.test(s.slice(m.index + m[0].length)) || /\d\s*%?\s*[–-]\s*$/.test(s.slice(Math.max(0, m.index - 8), m.index))) continue;  // 20–40%
    const after = phraseAfter(s, m.index + m[0].length, 6);
    const before = phraseBefore(s, m.index);
    const caption = /^(of|reduction|decrease|increase|lower|higher|fewer|more|less|improvement|accuracy|sensitivity|specificity|risk|success|response|survival|efficiency|yield|recall|precision|top-1|top-5|correct)/i.test(after)
      ? after.replace(/^of\s+(the\s+)?/i, "") : before || after;
    return { kind: "percent", value: v, caption: cap(clip(caption, 60)) || "Result" };
  }
  return null;
}

/* ---------- the main claim, in the authors' words ---------- */
const CLAIM_RE = /\b(?:here,? we (?:show|report|find|identify|discover|reveal|demonstrate)|we (?:show|find|found|report|identify|discover|reveal|demonstrate)|our (?:results|findings|data) (?:show|indicate|suggest|reveal)|these (?:results|findings) (?:show|indicate|suggest))\s+(?:that\s+)?/i;
// A review states what it covers instead of a finding: "Here we review how…", "This review summarises…".
const REVIEW_CLAIM_RE = /\b(?:here,? )?(?:we|this review|this article|the review)\s+(?:review|reviews|summari[sz]es?|discuss(?:es)?|highlights?|outlines?|describes?|examines?|explores?|synthesi[sz]es?|consider|considers|focus(?:es)? on|provides? an? (?:overview|summary|update) of)\s+/i;
function claim(abstract, key, isReview = false) {
  for (const s of [key, ...sections(abstract).filter(x => x.sec !== "background" && x.sec !== "other").map(x => x.s)]) {
    const m = s && (s.match(CLAIM_RE) || (isReview ? s.match(REVIEW_CLAIM_RE) : null));
    if (!m) continue;
    let c = s.slice(m.index + m[0].length).replace(/[.;]\s*$/, "").trim();
    if (c.length < 12) continue;
    if (c.length > 96) { c = c.slice(0, 96); c = c.slice(0, c.lastIndexOf(" ")) + "…"; }
    return cap(c);
  }
  return null;
}

/* ---------- named entity from the title ---------- */
const TERM_STOP = new Set(["DNA", "RNA", "AI", "ML", "LLM", "LLMs", "US", "USA", "UK", "EU", "II", "III", "IV", "VI", "COVID", "HIV", "SARS", "CI", "HR", "OR", "A", "AN"]);
function keyTerm(title) {
  const toks = String(title).replace(/[“”"'’:,;()[\]]/g, " ").split(/\s+/);
  for (const t of toks) {
    const w = t.replace(/[.]+$/, "");
    if (w.length < 2 || w.length > 18 || TERM_STOP.has(w)) continue;
    if (/^[A-Za-z][A-Za-z0-9-]*\d[A-Za-z0-9-]*$/.test(w) && /[A-Z]/.test(w)) return w;            // PCSK9, MCT1, SARS-CoV-2, Wee1
    if (/^[A-Z]{2,}[A-Za-z0-9-]*$/.test(w) && !/^[A-Z][a-z]+-[A-Z]/.test(w)) return w;             // CRISPR, ATP7B, LoL-align
    if (/^[A-Z][a-z]+[A-Z][A-Za-z0-9]+$/.test(w)) return w;                                       // AlphaGenome, MassNet
  }
  return null;
}

/* ---------- topic (picture) ---------- */
const TOPICS = [
  [/\b(crispr|base[- ]edit|prime[- ]edit|gene therap|genome editing)/i, "Gene editing", "dna"],
  [/\b(neurons?|neural circuits?|brain|cortex|cortical|synap|alzheimer|parkinson|dementia)/i, "Brain", "neuron"],
  [/\b(heart|cardiac|cardio|myocard|arter)/i, "Heart", "heart"],
  [/\b(virus|viral|bacteri|infection|pathogen|dengue|measles|influenza|covid|sars-cov|vaccin|antibiotic|malaria)/i, "Infection", "virus"],
  [/\b(cancer|tumou?r|oncolog|carcinoma|leuka?emia|lymphoma|metasta)/i, "Cancer", "cell"],
  [/\b(quantum|qubits?)\b/i, "Quantum", "atom"],
  [/\b(language models?|llms?|neural networks?|deep learning|machine learning|transformers?|reinforcement learning|ai agents?|generative)/i, "AI", "chip"],
  [/\b(rna|ribozymes?|chromatin|transcription factors?|ribosomes?)\b/i, "Molecular biology", "molecule"],
  [/\b(protein design|protein structure|enzymes?|antibod|binders?|peptides?)/i, "Proteins", "molecule"],
  [/\b(single-cell|transcriptom|proteom|metabolom|spatial|atlas|multi-omic)/i, "Omics", "bars"],
  [/\b(genom|sequencing|gwas|variants?|genetic)/i, "Genomics", "dna"],
  [/\b(diabet|obes|metabol|insulin|cholesterol|lipid)/i, "Metabolism", "pill"],
  [/\b(batter|solar|electrolyte|catalys|polymer|semiconductor|photonic|materials?)\b/i, "Materials & energy", "hex"],
  [/\b(public health|epidemiolog|outbreaks?|vaccination coverage|population-based|nationwide|global health|prevalence)/i, "Population health", "globe"],
  [/\b(hydrogels?|scaffolds?|biomaterials?|implants?|microfluidic|biosensors?|tissue engineering|bioprint)/i, "Bioengineering", "hex"],
  [/\b(climate|ocean|warming|atmospher|carbon|ecolog|species|biodiversity)/i, "Earth & climate", "globe"],
  [/\b(black holes?|galax|planet|stars?|dark matter|particle|cosmic|astro)/i, "Physics & space", "atom"],
  [/\b(software|compilers?|programs?|source code|vulnerab|security|bugs?)\b/i, "Software", "code"],
  [/\b(patients?|clinical|therap|treatment|hospital)/i, "Clinical", "pill"]
];
const FIELD_TOPIC = { bioeng: ["Bioengineering", "hex"], bioinfo: ["Bioinformatics", "bars"], biotech: ["Biotech", "dna"], cellmol: ["Cell biology", "cell"],
  medicine: ["Medicine", "pill"], omics: ["Omics", "bars"], publichealth: ["Public health", "globe"], none: ["Research", "atom"] };

export function summarize(s) {
  const title = normalise(s.title), abstract = normalise(s.abstract), text = `${title}. ${abstract}`;
  const [, topic, icon] = TOPICS.find(t => t[0].test(text)) || [null, ...(FIELD_TOPIC[s.field] || FIELD_TOPIC.none)];
  // The study size comes from what the authors did and found, not from background statistics.
  const studied = sections(abstract).filter(x => x.sec !== "background" && x.sec !== "other").map(x => x.s);
  const n = sampleSize([title, ...studied]);
  return {
    topic, icon, term: keyTerm(title),
    design: design(text, s.type, s.kind), subject: subject(text),
    n: n ? `${n.value} ${n.unit}` : null,
    figure: figure(title, abstract, normalise(s.keySentence || ""), n),
    claim: claim(abstract, normalise(s.keySentence || ""), s.kind === "Review")
  };
}

export const _internals = { sampleSize, figure, keyTerm, subject, design, fmt, sections };
export { fmt };
