import { query, mutation } from "./_generated/server";
import { v } from "convex/values";

/**
 * Convex Email Authentication Posture Queries & Mutations
 */

export const listEmailPostures = query({
  args: {},
  handler: async (ctx) => {
    const records = await ctx.db.query("emailAuthPosture").collect();
    // Join with assets table to resolve domain name
    const results = [];
    for (const rec of records) {
      const asset = await ctx.db.get(rec.domainAssetId);
      results.push({
        ...rec,
        domain: asset ? asset.value : "unknown-domain",
      });
    }
    return results;
  },
});

export const upsertEmailPosture = mutation({
  args: {
    domain: v.string(),
    spfValid: v.boolean(),
    dkimFound: v.boolean(),
    dmarcPolicy: v.optional(v.string()),
    priority: v.union(v.literal("critical"), v.literal("high"), v.literal("medium"), v.literal("low"), v.literal("info")),
  },
  handler: async (ctx, args) => {
    let asset = await ctx.db
      .query("assets")
      .withIndex("by_type_value", (q) => q.eq("type", "domain").eq("value", args.domain))
      .first();

    const now = Date.now();

    if (!asset) {
      const assetId = await ctx.db.insert("assets", {
        type: "domain",
        value: args.domain,
        source: "email-auth-checker",
        confidence: "high",
        status: "active",
        firstSeen: now,
        lastSeen: now,
      });
      asset = (await ctx.db.get(assetId))!;
    }

    const existing = await ctx.db
      .query("emailAuthPosture")
      .withIndex("by_domain", (q) => q.eq("domainAssetId", asset._id))
      .first();

    if (existing) {
      await ctx.db.patch(existing._id, {
        spf: { valid: args.spfValid },
        dkim: { found: args.dkimFound },
        dmarc: { policy: args.dmarcPolicy },
        priority: args.priority,
        checkedAt: now,
      });
      return existing._id;
    }

    return await ctx.db.insert("emailAuthPosture", {
      domainAssetId: asset._id,
      spf: { valid: args.spfValid },
      dkim: { found: args.dkimFound },
      dmarc: { policy: args.dmarcPolicy },
      priority: args.priority,
      checkedAt: now,
    });
  },
});
