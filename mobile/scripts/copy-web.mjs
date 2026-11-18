// Copies the shared web app into www/ and bundles the catalog as a module, so the app works offline-first
// (only the research sources themselves are fetched at open).
import { cpSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
const web = new URL("../../web/", import.meta.url), www = new URL("../www/", import.meta.url);
rmSync(www, { recursive: true, force: true });
mkdirSync(www, { recursive: true });
cpSync(web, www, { recursive: true });
writeFileSync(new URL("catalog.js", www), "export default " + readFileSync(new URL("catalog.json", web), "utf8") + ";\n");
console.log("web app copied to mobile/www");
