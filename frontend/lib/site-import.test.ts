import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  buildSiteImportTemplate,
  parseSiteImportJson,
  SITE_IMPORT_FORMAT,
  SiteImportError,
} from "./site-import.ts";

describe("site-import", () => {
  it("builds web and bot templates with format metadata", () => {
    const web = buildSiteImportTemplate("web");
    assert.equal(web.format, SITE_IMPORT_FORMAT);
    assert.equal(web.site_type, "web");
    assert.ok(web.primary_url);
    assert.ok(web.instructions);

    const bot = buildSiteImportTemplate("telegram_bot");
    assert.equal(bot.site_type, "telegram_bot");
    assert.ok(bot.secrets?.some((s) => s.key === "BOT_TOKEN"));
  });

  it("parses a filled website document and skips placeholder secrets", () => {
    const doc = buildSiteImportTemplate("web");
    doc.name = "Acme";
    doc.slug = "acme";
    doc.primary_url = "https://acme.example";
    doc.git_repo_url = "https://github.com/acme/site.git";
    doc.secrets = [
      { key: "GIT_TOKEN", value: "REPLACE_WITH_GITHUB_PAT_IF_PRIVATE" },
      { key: "API_KEY", value: "real-secret" },
    ];
    doc.deploy = false;

    const parsed = parseSiteImportJson(JSON.stringify(doc));
    assert.equal(parsed.siteType, "web");
    assert.equal(parsed.request.name, "Acme");
    assert.equal(parsed.request.primary_url, "https://acme.example");
    assert.equal(parsed.deploy, false);
    assert.deepEqual(parsed.secrets, { API_KEY: "real-secret" });
    assert.equal(parsed.request.nginx_ssl_enabled, true);
  });

  it("parses telegram bot and forces nginx off", () => {
    const raw = {
      format: SITE_IMPORT_FORMAT,
      format_version: 1,
      site_type: "telegram_bot",
      name: "Notify Bot",
      git_repo_url: "https://github.com/acme/bot.git",
      secrets: [{ key: "BOT_TOKEN", value: "123:abc" }],
    };
    const parsed = parseSiteImportJson(JSON.stringify(raw));
    assert.equal(parsed.siteType, "telegram_bot");
    assert.equal(parsed.request.nginx_ssl_enabled, false);
    assert.equal(parsed.request.nginx_force_https, false);
    assert.deepEqual(parsed.secrets, { BOT_TOKEN: "123:abc" });
    assert.equal(parsed.deploy, false);
  });

  it("rejects invalid JSON and missing website primary_url", () => {
    assert.throws(() => parseSiteImportJson("{"), SiteImportError);
    assert.throws(() => {
      parseSiteImportJson(
        JSON.stringify({
          site_type: "web",
          name: "X",
          git_repo_url: "https://github.com/a/b.git",
        }),
      );
    }, /primary_url/);
  });
});
