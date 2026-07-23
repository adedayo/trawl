import { mutation } from "./_generated/server";

/**
 * Convex Database Seeder
 * Populates initial database records directly in Convex if tables are empty.
 */
export const seedInitialDatabase = mutation({
  args: {},
  handler: async (ctx) => {
    const now = Date.now();

    // 1. Seed Config if missing
    let configDoc = await ctx.db.query("config").first();
    if (!configDoc) {
      await ctx.db.insert("config", {
        instanceName: "default",
        seedDomains: ["example.com", "api.example.com"],
        seedCidrs: ["198.51.100.0/24"],
        seedRepos: ["https://github.com/example/repo"],
        staleAfterDays: 90,
        triageThreshold: "high",
        secretVerificationEnabled: true,
        maxRepoCloneSizeMb: 500,
        aiProvider: {
          baseUrl: "https://api.openai.com/v1",
          model: "gpt-4o",
          timeoutMs: 30000,
        },
      });
    }

    // 2. Seed Assets if missing
    const existingAssets = await ctx.db.query("assets").collect();
    if (existingAssets.length === 0) {
      const a1 = await ctx.db.insert("assets", {
        type: "domain",
        value: "example.com",
        source: "seed",
        confidence: "high",
        status: "active",
        firstSeen: now - 86400000 * 3,
        lastSeen: now,
      });

      const a2 = await ctx.db.insert("assets", {
        type: "domain",
        value: "api.example.com",
        source: "subfinder",
        confidence: "high",
        status: "active",
        firstSeen: now - 86400000 * 2,
        lastSeen: now,
      });

      const a3 = await ctx.db.insert("assets", {
        type: "domain",
        value: "staging.example.com",
        source: "ct-logs",
        confidence: "medium",
        status: "pending",
        firstSeen: now - 86400000,
        lastSeen: now - 3600000,
      });

      const a4 = await ctx.db.insert("assets", {
        type: "ip",
        value: "198.51.100.42",
        source: "dns-pivot",
        confidence: "high",
        status: "active",
        firstSeen: now - 86400000 * 3,
        lastSeen: now,
      });

      const a5 = await ctx.db.insert("assets", {
        type: "repository",
        value: "https://github.com/example/repo",
        source: "operator",
        confidence: "high",
        status: "active",
        firstSeen: now - 86400000,
        lastSeen: now,
      });

      // 3. Seed Findings
      await ctx.db.insert("findings", {
        assetId: a2,
        cveIds: ["CVE-2024-3094"],
        kev: true,
        cvss: 10.0,
        priority: "critical",
        status: "open",
        statusHistory: [{ from: "open", to: "open", at: now }],
        dedupKey: "api.example.com::cve-2024-3094",
        firstSeen: now - 3600000 * 4,
        lastSeen: now,
      });

      await ctx.db.insert("findings", {
        assetId: a1,
        cveIds: ["CVE-2023-4863"],
        kev: true,
        cvss: 8.8,
        priority: "high",
        status: "open",
        statusHistory: [{ from: "open", to: "open", at: now }],
        dedupKey: "example.com::cve-2023-4863",
        firstSeen: now - 86400000,
        lastSeen: now,
      });

      await ctx.db.insert("findings", {
        assetId: a4,
        cveIds: ["CVE-2023-38408"],
        kev: false,
        cvss: 6.5,
        priority: "medium",
        status: "open",
        statusHistory: [{ from: "open", to: "open", at: now }],
        dedupKey: "198.51.100.42::cve-2023-38408",
        firstSeen: now - 86400000 * 2,
        lastSeen: now,
      });

      // 4. Seed Email Posture
      await ctx.db.insert("emailAuthPosture", {
        domainAssetId: a1,
        spf: { valid: true },
        dkim: { found: true },
        dmarc: { policy: "reject" },
        priority: "info",
        checkedAt: now,
      });

      await ctx.db.insert("emailAuthPosture", {
        domainAssetId: a2,
        spf: { valid: true },
        dkim: { found: false },
        dmarc: { policy: "quarantine" },
        priority: "medium",
        checkedAt: now,
      });

      await ctx.db.insert("emailAuthPosture", {
        domainAssetId: a3,
        spf: { valid: false },
        dkim: { found: false },
        dmarc: { policy: "none" },
        priority: "high",
        checkedAt: now,
      });

      // 5. Seed Secret Findings
      await ctx.db.insert("secretFindings", {
        repoAssetId: a5,
        filePath: "config/example-credentials.json",
        provider: "AWS IAM Key",
        redactedRef: "AKIA...8F2A (REDACTED:SHA256)",
        commitSha: "67c049e",
        lastScannedSha: "67c049e",
        verified: false,
        priority: "high",
        status: "open",
        firstSeen: now - 3600000 * 6,
        lastSeen: now,
      });
    }

    return { success: true, seededAt: now };
  },
});
