# Research Digest

A live research newsletter for seven fields, listed alphabetically: Bioengineering, Bioinformatics, Biotechnology, Cellular and Molecular Biology, Medicine, Omics and Public Health. Every time you open it, it reads the sources fresh, ranks what it finds, and shows one scrolling column of articles. Papers are never stored; only your own categories (keyword lists) are saved.

**No keys, accounts or sign-ups.** It reads only free, open sources: Europe PMC (37 journals, via PubMed, including Circulation and Skeletal Muscle), bioRxiv, medRxiv and arXiv.

## Version 1.0.0: Windows, Mac, Linux and Android

| Computer or phone | File |
|---|---|
| Windows (most PCs) | `ResearchDigest-windows-x64.zip` (contains `ResearchDigest.exe`) |
| Windows on ARM (e.g. Snapdragon laptops) | `ResearchDigest-windows-arm64.zip` |
| Mac (Intel and Apple Silicon, macOS 11 or later) | `ResearchDigest-mac.zip` |
| Linux (most PCs) | `ResearchDigest-linux-x64.zip` or `.tar.gz` (program, icon, menu entry and installer) |
| Linux on ARM (e.g. Raspberry Pi 4/5, 64-bit) | `ResearchDigest-linux-arm64.zip` or `.tar.gz` |

Programs come in `.zip` files because many browsers, chat apps and email services refuse to pass on bare `.exe` files.
| Android | `ResearchDigest-android.apk`, built by the included GitHub workflow (see below) |
| iPhone / iPad | built with Xcode on a Mac (see below) |

## Desktop (Windows, Mac and Linux)

Open the program and Research Digest opens in your browser. **No terminal window opens**: the program runs quietly in the background while its page is open.

- **Several tabs are fine:** only one tab at a time keeps the program's "page is open" connection (browsers allow just 6 connections to a site, shared by all its tabs), handing it to another tab when it closes, so many open tabs never slow loading.
- **It closes with its page:** close the tab or the browser window and the program stops about 5 seconds later (the short wait lets a reload keep it running). **Quit** at the top left of the page stops it at once. A tab left open keeps it running, even in the background, and with two tabs open it stops when the last one closes.
- **Opening it again while it is running** opens another tab of the same copy rather than starting a second one.
- **Windows:** unzip, then double-click `ResearchDigest.exe` (you can move it anywhere, e.g. your Desktop). Windows may show "Windows protected your PC" because the file is not code-signed: click **More info → Run anyway**. The file has an open-book icon in journal red and ivory; if Windows still shows an older icon after you replace the file, check **Properties → Details** (File version 1.0.0 means you have this file, and Windows is showing a saved copy of the old icon): press Win+R and run `ie4uinit.exe -show`, or move the file to another folder.
- **Mac:** unzip `ResearchDigest-mac.zip` and drag **Research Digest** to Applications. It is not from the App Store, so the first time macOS will say it cannot check it: open **System Settings → Privacy & Security**, scroll down and click **Open Anyway** (on older macOS, right-click the app and choose **Open**). If macOS says the app "is damaged", that is the same check: in Terminal, run `xattr -cr "/Applications/ResearchDigest.app"` and open it again. It has no Dock icon; it lives in the browser tab.
- **Linux:** unzip (or unpack the `.tar.gz`) and run `./install.sh` inside it. This puts Research Digest, with its icon, in your applications menu (no root needed; `./install.sh --uninstall` removes it). Or run the bare file directly: `chmod +x ResearchDigest-linux-x64 && ./ResearchDigest-linux-x64`.

Options, for starting it from a terminal or shortcut: `-port 8787` (a free port is chosen if it is taken), `-no-browser`, and `-lan` to also serve phones and tablets on the same Wi-Fi (the page then shows the address to open on the phone). With `-lan` or `-no-browser` it keeps running until you press **Quit**, since others may be using it. Only the computer running it is offered **Quit**.

Each file is a single program of about 7 MB with the app built in. Nothing else to install.

## Mobile (Android and iOS)

The mobile app runs the same engine on the phone itself, so it needs no computer or server.

- **Android:** install `ResearchDigest-android.apk`. Android will ask to allow installs from your browser or file manager the first time.
- **iOS:** Apple only allows apps from the App Store or built with Xcode. On a Mac with Xcode and CocoaPods (`brew install cocoapods`): `cd mobile && npm install && npm run ios && npx cap open ios`, then press Run with your iPhone connected (a free Apple ID works for your own device).
- **Any phone, no install:** run the desktop program with `-lan`, open the printed address on the phone, then use **Add to Home Screen**.

### Building the Android app

The APK is built by the included GitHub workflow: push this folder to a GitHub repository, open the **Actions** tab, and download `ResearchDigest-android.apk` from the finished run. The same run also rebuilds the Windows, Linux and Mac files and checks the iOS project compiles. The same run also builds the Mac app on a Mac (so it is signed ad hoc there). Tag a commit `v1.0.0` to publish everything as a release.

To build locally instead (needs Node 20+, JDK 21 and Android Studio's SDK):

```bash
cd mobile
npm install
npm run android        # creates the Android project with the app icon
npm run android:apk    # → android/app/build/outputs/apk/debug/app-debug.apk
```

## How it ranks

| Window | Reads | First cut |
|---|---|---|
| Today (48 h) | top and specialist journals, bioRxiv, medRxiv, arXiv | newest |
| This week | same, with preprint pages sampled across the week | newest |
| This month | journals and preprints via Europe PMC | most cited |
| This year | same | most cited |
| Past 5 years | same | most cited |
| All time | same, with no date limit | most cited |

Each range shows its 20 best papers (change this with **Papers per range** in the Show row, from 5 to 200; it is remembered on each device, and more candidates are fetched when you ask for over 75). The number is exact. Within it, every field and every paper type (peer reviewed or preprint) found among the range's candidates keeps at least one place, taking the place of the lowest-ranked paper of a field and type that appears more than once (at most a third of the places change). So a filter never comes up empty in a longer range, for example All time, when a shorter one such as Past 5 years has a match, even though All time is mostly long-established, highly cited work. Searches and your categories work the same way. The counts always add up: **All papers** lists everything in the tab, and **Peer reviewed** plus **Preprints** split that same list, as do the field chips.

Longer ranges contain the shorter ones: whatever Today, This week or This month finds is also a candidate for This year, Past 5 years and All time, on top of each range's own fetch. Limits per journal and per field (so one source cannot fill a tab) only apply when there are more papers than room.

Each paper gets a score from three signals:

- **Where it was published:** leading journals first, then specialist journals, then preprints (a preprint since published in a journal counts higher).
- **How new it is** (today, this week) or **how often it is cited per month since publication** (this month and longer), so recent papers are not judged against ones cited for years.
- **Whether the abstract reports a concrete finding**, such as "we show", a randomized or phase 3 trial, a first demonstration, or an atlas.

Each article is shown Medium-style: journal, title, the abstract sentence stating the main result, date and field, with a summary card on the right (below the text on phones). The card is read from the paper's own abstract:

- **Headline figure:** two results side by side (e.g. median survival 21.1 vs 12.7 months, or before/after), a percentage ring, a fold change, a hazard/odds ratio or AUC on a small scale, or the study size. If the abstract has no number, the card shows the main finding in the authors' words.
- **Facts:** how the study was done (phase 3 trial, cohort, single-cell atlas, new method…), what was studied (humans, mice, cells, computational…) and how many.
- **Topic and key term:** for example Gene editing · PCSK9.

Numbers are only ever quoted from the abstract, never estimated. They are taken from the Results and Conclusions (or, in an abstract without headings, from the result sentences after its opening background sentence), never from the background, the study design, funding or registration lines. Within a sentence, ranges, confidence intervals, study power, demographics such as "54% were women", scientific notation and dosing are skipped, ratio abbreviations count only in capitals (HR, OR, so "or" and "hr" never do), negative values keep their sign, thousands written with spaces or thin spaces ("69 521 genes") are read as one number, and units such as mm, mg/dL and mm Hg are recognised. The line under each title is likewise the first result sentence.

**All papers / Peer reviewed / Preprints / Reviews** filters every tab, field, search and category (see Paper types).

Corrections, errata, editorials and replies are removed, duplicates and older preprint versions are merged, and no field or journal may take more than a quarter or a fifth of the list, so all seven fields appear. Each card's quoted line is the abstract sentence that states the main result, in the authors' own words (the approach [Pipette](https://github.com/ferced/pipette) uses), so no text is generated.

The journal list, arXiv categories, field rules and cue words live in `web/catalog.json`, shared by the desktop and mobile versions.

## Layout

- The title is centred, with a **Dark / Light** switch at the top right. Research Digest opens in the light theme; if you switch to dark, that choice is remembered on that device.
- **Search and filters** sit in one panel: a search box and the search tips, then centred rows for **Field**, **My categories** (with **+ New category**) and **Show** (All papers / Peer reviewed / Preprints and **Sources**), each with its label above. The centred time tabs stay pinned to the top while you scroll, with the number of results and **Reload** centred on the line under them.
- **Style:** a printed-journal look: ivory paper, a newspaper-style nameplate between ruled lines, Libre Baskerville headlines, Lora for summaries and abstracts (italic summary lines, a drop capital opening each abstract), Inter for labels and buttons, and a deep journal red for links and the selected tab. Dark mode keeps the same layout in warm charcoal and cream.
- **Reading:** abstracts are set at a comfortable size and line spacing in slightly softer ink. Structured abstracts (Background, Methods, Results, Conclusions…) are shown as short labelled paragraphs, whether the source marks the sections with HTML headings (Europe PMC), capitals (BACKGROUND …) or colons.
- Each article's title, summary line and abstract use the full width up to its summary card, and the notes at the bottom fill the page width.

## Search

Type keywords in the search bar (use quotes for an exact phrase, e.g. `"spatial transcriptomics" mouse`), an author, or both, and press Search. A search works like a category you have not saved yet: it is pulled live for every time tab, so **Today, This week, This month, This year, Past 5 years and All time** each show that search's results for that range, with a count on each tab. Switching tabs keeps the search. The field chips and All papers / Peer reviewed / Preprints narrow the results too. A bar under the search box shows what you searched, with **☆ Save as category** to keep it and **× Clear** to go back to the main digest.

The search runs live through Europe PMC across the 37 journals in the digest, every other journal it indexes, and the bioRxiv, medRxiv and arXiv preprints, plus arXiv's own search. Your keywords must appear in a paper's title or abstract. Within each tab, papers rank the way the main tabs do (newer first for Today and This week, more cited per month for longer ranges) plus how closely they match. Under each title, the grey line is the abstract sentence that mentions your words most.

## Paper types: peer reviewed, preprints and reviews

The **Show** row splits every tab, search and category into three types:

- **Peer reviewed**: original research published in a journal.
- **Preprints**: research shared on bioRxiv, medRxiv or arXiv before peer review.
- **Reviews**: review articles, which summarise and interpret published research rather than report a new study. They are labelled **Review** on a red tag (Peer reviewed is green, Preprint yellow) and get a small ranking penalty, so heavily cited reviews don't crowd out new findings.

**All papers** lists all three, and the counts always add up: All papers = Peer reviewed + Preprints + Reviews. Untick **Include reviews** to leave reviews out of every tab, search and category; they are then left out before ranking, so each range still lists exactly its number of papers. Both choices are remembered on each device.

### What appears

Every tab, search and category shows the same kinds of paper: original peer-reviewed studies, preprints and reviews, each with a real abstract. Editorials, news, comments, letters, corrections and short items without an abstract are left out everywhere. Reviews get the same summary card as other papers, read from their own abstract: it says **Review** and states what the review covers (or quotes a number from its results, for example from a meta-analysis).

A new search shows every field. If a field chip or a paper type leaves a tab with nothing, it says how many other papers the tab has and offers **Show all**, rather than looking empty.

### Searching by author

- A name on its own, such as `Jennifer Doudna`, `J. A. Doudna` or `Doudna JA`, is searched both as an author and as keywords; that author's papers come first, newest first.
- `author:`, `au:` or `by:` searches authors only: `author:"Feng Zhang"`, `by:Doudna`. Combine with keywords to narrow: `CRISPR author:Doudna`. Up to four authors can be given; all must be on the paper.
- People known by a middle name match too: "Jeffrey Dilworth" finds "Dilworth FJ" (F. Jeffrey Dilworth), because a typed initial matches if it appears among the author's initials in order.
- Indexes write initials differently ("Doudna J" or "Doudna JA"), so the digest's journals are searched both by surname and initials and by surname alone. Either way, names are matched against each paper's own author list, by surname and first initial, ignoring accents, so someone merely mentioned in an abstract never counts. Each result shows the author as the paper writes the name and their position, for example "Doudna JA · last of 12 authors".
- Author searches work in your own categories too, so you can follow a researcher across every time range.

## Your own categories

Next to the seven fields, **+ New category** turns keywords into a category of its own, for example `CRISPR delivery` or `"organoids" kidney`. Selecting it pulls matching papers live from every source (the digest's journals, other journals in Europe PMC, bioRxiv, medRxiv and arXiv) for every time range from Today to All time, so it is never limited to papers already on screen. Within each window it ranks the way the main tabs do: newer papers for today and this week, more-cited papers for the longer ranges, plus how closely each paper matches your keywords. A search can be kept with **☆ Save as category**, and × removes a category.

Your **Sources** selection, **Papers per range** and **Include reviews** are saved between sessions too, the same way: on desktop in `settings.json` next to the categories file (so they survive restarts, a change of port and a different browser), in the mobile app in its own storage.

Categories are saved between sessions. On desktop they go in a small file on your computer, so they survive restarts, a change of port and a different browser: `%AppData%\ResearchDigest\categories.json` on Windows, `~/.config/ResearchDigest/categories.json` on Linux (the program prints the path when it starts). In the mobile app they are kept in the app's own storage. Only the keywords are saved; their papers are always fetched fresh. On the search tab, **☆ Save as category** keeps a search; it then shows **✓ Saved category**.

## Speed

- Desktop: Go, standard library only. All sources for a range are fetched at once, and all six ranges load at once when the app opens. A source that is slow (15 s limit) or down is skipped and named in the status line.
- If two windows or people request the same feed at the same moment, one fetch serves both. Nothing is written to disk.
- Responses are gzip-compressed; a full open is typically a few seconds, mostly waiting on the sources.

## Project layout

```
main.go, server.go, sources.go, rank.go, catalog.go, search.go, query.go, categories.go   desktop program (Go)
build/          app icon (PNG, ICO), Windows resources, Linux menu entry and installer
web/            the app's screens (shared) and engine.js, the on-phone engine
mobile/         Capacitor project for Android and iOS
tests/          on-phone engine tests       main_test.go  desktop tests
.github/workflows/build.yml                 builds everything
```

Run the tests with `go test ./...` and `node --test tests/engine.test.mjs`. Build the desktop files yourself with Go 1.22+: `GOOS=windows GOARCH=amd64 go build -o ResearchDigest.exe .` (the icon comes from the `rsrc_windows_*.syso` files, made from `build/icon/` by `build/make-windows-resources.sh`). `sh build/package-linux.sh` packs the Linux builds in `dist/` with the icon and installer.

## Notes

- Journal papers reach Europe PMC once PubMed lists them, usually within a few days of publication, so the very newest journal papers may appear a little later than on the journal's site. New preprints appear the same day, read straight from bioRxiv, medRxiv and arXiv.
- Why not OpenAlex: since February 2026 OpenAlex needs an API key for real use (keyless use is capped at about 10 list queries a day), so the digest uses Europe PMC, which needs none.
- Preprints are labelled and have not completed peer review.

## Fields

Each paper is placed in one of the seven fields from its journal (for example *Lab on a Chip* → Bioengineering, *The Lancet Public Health* → Public Health) or, for broad journals such as *Nature*, *Science* and preprint servers, from words in its title and abstract. The rules are in `web/catalog.json`. Papers that fit none of the seven, such as physics or computer science, are left out of the main tabs; they can still appear in searches and your own categories.

## Sources

**Sources** (next to All papers / Peer reviewed / Preprints) lists everything Research Digest reads, grouped as preprint servers, leading journals and specialist journals, with the number of papers each contributes to the current tab. Tick or untick sources and the results update by themselves a moment later (**Done** closes the panel): every tab, search and category is then fetched from the ticked sources only, not just filtered. Journals added to Research Digest after you made a selection are ticked automatically. **Other journals in PubMed** is used by searches and categories only. The choice is remembered on this device.

## Renamed from Daily Digest

The app is now called Research Digest. Categories saved under the old name are picked up automatically and written to the new folder (`ResearchDigest`) the next time you change them.
