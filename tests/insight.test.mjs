import test from "node:test";
import assert from "node:assert/strict";
import { summarize } from "../web/insight.js";

const S = (title, abstract, extra = {}) => summarize({ title, abstract, field: "medicine", type: "journal", ...extra });

test("comparison of two arms with labels", () => {
  const r = S("Tarlatamab plus durvalumab in extensive-stage small-cell lung cancer",
    "Background text here. In this phase 3 trial, 563 patients were randomly assigned. The median overall survival was 21.1 months with tarlatamab–durvalumab and 12.7 months with durvalumab alone. Adverse events were manageable.");
  assert.equal(r.figure.kind, "compare");
  assert.equal(r.figure.a.value, 21.1); assert.equal(r.figure.b.value, 12.7);
  assert.equal(r.figure.unit, "months");
  assert.match(r.figure.caption, /median overall survival/i);
  assert.match(r.figure.a.label, /tarlatamab/); assert.match(r.figure.b.label, /durvalumab/);
  assert.equal(r.design.label, "Phase 3 trial"); assert.equal(r.subject, "Humans"); assert.equal(r.n, "563 patients");
  assert.equal(r.topic, "Cancer");
});

test("before/after change", () => {
  const r = S("CGM in type 2 diabetes", "In 283 adults not using insulin, HbA1c decreased from 8.8% to 7.2% with continuous glucose monitoring. Control fell less.");
  assert.equal(r.figure.kind, "compare"); assert.equal(r.figure.a.label, "Before"); assert.equal(r.figure.b.value, 7.2); assert.equal(r.figure.unit, "%");
});

test("percent result with caption", () => {
  const r = S("In vivo base editing of PCSK9 with VERVE-102", "A single infusion was given. At the highest dose, the treatment reduced LDL cholesterol by 62% and PCSK9 by 88% (95% CI, 80 to 92).");
  assert.equal(r.figure.kind, "percent"); assert.equal(r.figure.value, 62);
  assert.match(r.figure.caption, /reduced LDL cholesterol/i);
  assert.equal(r.term, "PCSK9");
});

test("fold change", () => {
  const r = S("Perinatal elimination of genetically aberrant human cortical neurons", "We sequenced 1,526 neurons from 16 individuals. The fraction of aberrant neurons fell 4.5-fold after birth.", { field: "cellmol" });
  assert.equal(r.figure.kind, "fold"); assert.equal(r.figure.value, 4.5);
  assert.equal(r.n, "1,526 neurons"); assert.equal(r.topic, "Brain");
});

test("hazard ratio and AUC", () => {
  assert.equal(S("x", "Death occurred less often (hazard ratio, 0.42; 95% CI, 0.31 to 0.57) in the treated group.").figure.name, "Hazard ratio");
  const r = S("A deep learning model for sepsis", "The model achieved an AUC of 0.91 in external validation across 12 hospitals.", { field: "software" });
  assert.equal(r.figure.kind, "ratio"); assert.equal(r.figure.name, "AUC"); assert.equal(r.figure.value, 0.91);
});

test("falls back to sample size, then nothing; ignores years and CIs", () => {
  const r = S("Atlas of the developing heart", "We profiled 120,000 cells from 2019 to 2023 using single-cell sequencing to build an atlas.", { field: "omics" });
  assert.equal(r.figure.kind, "count"); assert.equal(r.figure.value, "120,000"); assert.equal(r.design.label, "Single-cell atlas");
  const q = S("A theory of things", "We derive an analytical model of the phenomenon and discuss implications.", { field: "science" });
  assert.equal(q.figure, null); assert.equal(q.design.label, "Theory & simulation");
});

test("Lancet decimal point and key terms", () => {
  const r = S("AlphaGenome predicts variant effects", "Accuracy improved to 94·5% across benchmarks.", { field: "bioinfo" });
  assert.equal(r.figure.value, 94.5); assert.equal(r.term, "AlphaGenome");
  assert.equal(S("Error-Corrected Memory and Logic", "x").term, null);
});

test("main claim when there is no number", () => {
  const r = S("A small ribozyme that copies itself", "The origin of self-replication is unresolved. Here we show that QT45, a small polymerase ribozyme, synthesizes both itself and its complementary strand. This suggests simpler replicators.", { field: "cellmol" });
  assert.equal(r.figure, null);
  assert.match(r.claim, /^QT45, a small polymerase ribozyme, synthesizes/);
  assert.equal(r.design.label, "New finding");
});
test("second arm label keeps its full phrase", () => {
  const r = S("x", "Median survival was 21.1 months with tarlatamab–durvalumab and 12.7 months with durvalumab alone, respectively.");
  assert.equal(r.figure.b.label, "durvalumab alone");
});

/* ---------- abstracts written the way journals write them ---------- */

test("structured abstract: the figure comes from Results, not a Background statistic", () => {
  const r = S("Early antibiotics for sepsis in emergency departments",
    "Background: Sepsis kills 20% of patients admitted to hospital worldwide, and 49 million people are affected each year. Methods: We enrolled 1,204 adults in 12 hospitals; the trial was powered at 90% to detect a difference. Results: Mortality at 28 days was 18.2% with early antibiotics and 24.6% with usual care. Conclusions: Early antibiotics reduced mortality.");
  assert.equal(r.figure.kind, "compare");
  assert.deepEqual([r.figure.a.value, r.figure.b.value], [18.2, 24.6]);
  assert.equal(r.n, "1,204 adults");   // not the 49 million in the background
});

test("hazard ratio in Results wins over percentages in Methods", () => {
  const r = S("Semaglutide and kidney outcomes",
    "Objective: To assess kidney outcomes. Design, Setting, and Participants: Of 3,533 participants, 69% were men; 80% power was assumed. Results: The risk of the primary outcome was lower with semaglutide (hazard ratio, 0.76; 95% CI, 0.66 to 0.88). Conclusions and Relevance: Semaglutide reduced kidney events.");
  assert.equal(r.figure.kind, "ratio"); assert.equal(r.figure.name, "Hazard ratio"); assert.equal(r.figure.value, 0.76);
});

test("the word 'or' and hours ('hr') are not ratios", () => {
  const r = S("Sleep and memory", "Participants slept for 7.5 hr or 4.5 hr per night across 2 weeks. Recall improved by 23% after full sleep compared with restriction.");
  assert.notEqual(r.figure.kind, "ratio");
  assert.equal(r.figure.kind, "percent"); assert.equal(r.figure.value, 23);
});

test("odds ratio written as OR is still found", () => {
  const r = S("Smoking and stroke", "We analysed 12,000 cases. Smoking was associated with stroke (OR 1.84, 95% CI 1.52-2.21) after adjustment.");
  assert.equal(r.figure.kind, "ratio"); assert.equal(r.figure.name, "Odds ratio"); assert.equal(r.figure.value, 1.84);
});

test("scientific notation and dosing are not fold changes", () => {
  const r = S("A CAR-T dose study", "Patients received 5 x 10^6 cells per kilogram twice, and 2 times daily oral steroids. Complete response increased to 64% of patients.");
  assert.notEqual(r.figure.kind, "fold");
  assert.equal(r.figure.kind, "percent"); assert.equal(r.figure.value, 64);
});

test("demographics and age ranges are not results", () => {
  const r = S("Vitamin D in older adults", "Participants aged from 65 to 85 years were included; 54% were women. Falls were reduced by 12% with vitamin D compared with placebo.");
  assert.equal(r.figure.kind, "percent"); assert.equal(r.figure.value, 12);
});

test("ranges of percentages are skipped", () => {
  const r = S("Antibody prevalence", "Seroprevalence ranged from 20% to 40% across regions. Vaccination increased protection by 71% against hospitalisation.");
  assert.equal(r.figure.value, 71);
});

test("negative changes keep their sign", () => {
  const r = S("A depression trial", "In 410 adults, the mean change in score was −8.1 points with drug and −3.2 points with placebo.");
  assert.equal(r.figure.kind, "compare"); assert.deepEqual([r.figure.a.value, r.figure.b.value], [-8.1, -3.2]);
});

test("unstructured abstract: the opening background sentence is not used", () => {
  const r = S("Gut bacteria and obesity",
    "Obesity affects 40% of adults, and rates increased 3-fold since 1975. Here we show that a single bacterial strain protects mice from weight gain. Colonised mice gained 35% less weight than germ-free controls.");
  assert.equal(r.figure.kind, "percent"); assert.equal(r.figure.value, 35);
});

test("funding and registration lines are ignored", () => {
  const r = S("A trial of drug X", "In 800 patients, drug X lowered blood pressure. (Funded by Pharma; ClinicalTrials.gov number, NCT01234567; 95% of funding.)");
  assert.notEqual(r.figure && r.figure.value, 95);
});

test("sections are read from labelled abstracts", async () => {
  const { _internals } = await import("../web/insight.js");
  const parts = _internals.sections("Background: Sepsis is common. Methods: We enrolled adults. Results: Mortality fell. Conclusions: It works.").map(x => x.sec);
  assert.deepEqual(parts, ["background", "methods", "results", "conclusions"]);
});

test("measurements with units such as mm, mg/dL and mm Hg are compared", () => {
  let r = S("A cartilage scaffold", "In 24 rats, the implant restored cartilage thickness from 0.4 mm to 1.1 mm after eight weeks.");
  assert.equal(r.figure.kind, "compare"); assert.deepEqual([r.figure.a.value, r.figure.b.value, r.figure.unit], [0.4, 1.1, "mm"]);
  r = S("A blood pressure trial", "In 1,000 adults, systolic pressure was 128 mm Hg with treatment and 135 mm Hg with placebo, a reduction.");
  assert.equal(r.figure.unit, "mm Hg");
  r = S("Gene counts", "Expression rose from 10 to 12 genes per cell across 400 samples.");
  assert.notEqual(r.figure && r.figure.unit, "g");
});

test("thousands written with spaces or thin spaces are one number", () => {
  let r = S("A gene atlas", "We profiled 69 521 genes across 1 204 samples. Expression rose 3-fold in tumours.");
  assert.equal(r.n, "69,521 genes");
  r = S("A trial", "Of 12 345 patients enrolled, mortality was lower with treatment.");
  assert.equal(r.n, "12,345 patients");
  r = S("Responses", "At week 4 100% of 300 patients responded with improved scores.");   // "4 100%" is not 4,100
  assert.equal(r.figure.value, 100);
});

test("reviews get the usual summary card: labelled Review, with what the review covers as its main point", () => {
  const r = S("Enhancers integrate microenvironmental signals in muscle stem cells during regeneration in health, disease, and aging",
    "Effective skeletal muscle regeneration requires muscle stem cells (MuSCs) to continuously interpret signals from their surroundings. These niche-derived cues guide MuSCs through quiescence, activation, proliferation and differentiation by reshaping gene expression. Here we review how enhancer landscapes change in muscle stem cells during regeneration, in muscular dystrophies and with aging.",
    { kind: "Review" });
  assert.equal(r.design.label, "Review");
  assert.match(r.claim, /^How enhancer landscapes change in muscle stem cells/);
  const sr = S("Statins and stroke: a systematic review", "We searched five databases. Across 40 trials and 120,000 participants, statins reduced stroke by 18% compared with placebo.", { kind: "Review" });
  assert.equal(sr.figure.kind, "percent");   // numbers from a review's own results are shown like any other paper's
});
