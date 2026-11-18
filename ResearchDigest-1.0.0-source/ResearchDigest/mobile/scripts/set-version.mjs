// Gives the generated Android and iOS projects this app's version (package.json), e.g. 1.0.0,
// instead of Capacitor's default 1.0.
import { readFileSync, writeFileSync, existsSync } from "node:fs";
const version = JSON.parse(readFileSync(new URL("../package.json", import.meta.url))).version;
const [maj, min, pat] = version.split(".").map(Number), code = maj * 10000 + min * 100 + pat;
const edit = (path, f) => { if (existsSync(path)) { writeFileSync(path, f(readFileSync(path, "utf8"))); console.log("Version", version, "→", path); } };
edit("android/app/build.gradle", s => s.replace(/versionName\s+"[^"]*"/, `versionName "${version}"`).replace(/versionCode\s+\d+/, `versionCode ${Math.max(1, code)}`));
edit("ios/App/App.xcodeproj/project.pbxproj", s => s.replace(/MARKETING_VERSION = [^;]+;/g, `MARKETING_VERSION = ${version};`));
