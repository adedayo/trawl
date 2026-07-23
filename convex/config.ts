import { query, mutation } from "./_generated/server";
import { v } from "convex/values";

/**
 * Convex Configuration & Scope Management Functions
 */

// Query: Fetch instance configuration and scope authorization state
export const getConfig = query({
  args: {},
  handler: async (ctx) => {
    const configDoc = await ctx.db.query("config").first();
    if (!configDoc) {
      return {
        instanceName: "default",
        seedDomains: ["example.com"],
        seedCidrs: [],
        seedRepos: [],
        staleAfterDays: 90,
        triageThreshold: "high" as const,
        secretVerificationEnabled: true,
        maxRepoCloneSizeMb: 500,
        aiProvider: {
          baseUrl: "https://api.openai.com/v1",
          model: "gpt-4o",
          timeoutMs: 30000,
        },
        authorizationSignedAt: undefined,
        authorizationSigner: undefined,
        authorizationRulesVersion: undefined,
      };
    }
    return configDoc;
  },
});

// Mutation: Digitally sign scope authorization
export const signAuthorization = mutation({
  args: {
    signerName: v.string(),
    signerTitle: v.optional(v.string()),
    rulesVersion: v.optional(v.string()),
  },
  handler: async (ctx, args) => {
    const configDoc = await ctx.db.query("config").first();
    const signedAt = Date.now();
    const rulesVer = args.rulesVersion || "v1.0";

    if (configDoc) {
      await ctx.db.patch(configDoc._id, {
        authorizationSignedAt: signedAt,
        authorizationSigner: `${args.signerName}${args.signerTitle ? ` (${args.signerTitle})` : ""}`,
        authorizationRulesVersion: rulesVer,
      });
    } else {
      await ctx.db.insert("config", {
        instanceName: "default",
        seedDomains: ["example.com"],
        seedCidrs: [],
        seedRepos: [],
        staleAfterDays: 90,
        triageThreshold: "high",
        secretVerificationEnabled: true,
        maxRepoCloneSizeMb: 500,
        aiProvider: {
          baseUrl: "https://api.openai.com/v1",
          model: "gpt-4o",
          timeoutMs: 30000,
        },
        authorizationSignedAt: signedAt,
        authorizationSigner: `${args.signerName}${args.signerTitle ? ` (${args.signerTitle})` : ""}`,
        authorizationRulesVersion: rulesVer,
      });
    }

    return { success: true, signedAt };
  },
});

// Mutation: Revoke scope authorization
export const revokeAuthorization = mutation({
  args: {},
  handler: async (ctx) => {
    const configDoc = await ctx.db.query("config").first();
    if (configDoc) {
      await ctx.db.patch(configDoc._id, {
        authorizationSignedAt: undefined,
        authorizationSigner: undefined,
        authorizationRulesVersion: undefined,
      });
    }
    return { success: true };
  },
});

// Mutation: Update target scope (seed domains, CIDRs, repos)
export const updateScopeTargets = mutation({
  args: {
    seedDomains: v.array(v.string()),
    seedCidrs: v.array(v.string()),
    seedRepos: v.array(v.string()),
  },
  handler: async (ctx, args) => {
    const configDoc = await ctx.db.query("config").first();
    if (configDoc) {
      await ctx.db.patch(configDoc._id, {
        seedDomains: args.seedDomains,
        seedCidrs: args.seedCidrs,
        seedRepos: args.seedRepos,
      });
    } else {
      await ctx.db.insert("config", {
        instanceName: "default",
        seedDomains: args.seedDomains,
        seedCidrs: args.seedCidrs,
        seedRepos: args.seedRepos,
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

    const now = Date.now();

    // Auto-create active assets for any new seed domains not yet in database
    for (const domain of args.seedDomains) {
      const existing = await ctx.db
        .query("assets")
        .withIndex("by_type_value", (q) => q.eq("type", "domain").eq("value", domain))
        .first();

      if (!existing) {
        await ctx.db.insert("assets", {
          type: "domain",
          value: domain,
          source: "seed",
          confidence: "high",
          status: "active",
          firstSeen: now,
          lastSeen: now,
        });
      }
    }

    // Auto-create active assets for any new seed repos not yet in database
    for (const repo of args.seedRepos) {
      const existing = await ctx.db
        .query("assets")
        .withIndex("by_type_value", (q) => q.eq("type", "repository").eq("value", repo))
        .first();

      if (!existing) {
        await ctx.db.insert("assets", {
          type: "repository",
          value: repo,
          source: "seed",
          confidence: "high",
          status: "active",
          firstSeen: now,
          lastSeen: now,
        });
      }
    }

    // Auto-create active assets for any new seed CIDRs not yet in database
    for (const cidr of args.seedCidrs) {
      const existing = await ctx.db
        .query("assets")
        .withIndex("by_type_value", (q) => q.eq("type", "ip").eq("value", cidr))
        .first();

      if (!existing) {
        await ctx.db.insert("assets", {
          type: "ip",
          value: cidr,
          source: "seed",
          confidence: "high",
          status: "active",
          firstSeen: now,
          lastSeen: now,
        });
      }
    }

    return { success: true };
  },
});
