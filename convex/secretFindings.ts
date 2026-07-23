import { query, mutation } from "./_generated/server";
import { v } from "convex/values";

/**
 * Convex Secret Scanning Queries & Mutations
 */

export const listSecretFindings = query({
  args: {},
  handler: async (ctx) => {
    const records = await ctx.db.query("secretFindings").collect();
    const results = [];
    for (const rec of records) {
      const asset = await ctx.db.get(rec.repoAssetId);
      results.push({
        ...rec,
        repoUrl: asset ? asset.value : "unknown-repo",
      });
    }
    return results;
  },
});

export const upsertSecretFinding = mutation({
  args: {
    repoUrl: v.string(),
    filePath: v.string(),
    provider: v.string(),
    redactedRef: v.string(),
    commitSha: v.string(),
    verified: v.boolean(),
    priority: v.union(v.literal("critical"), v.literal("high"), v.literal("medium"), v.literal("low")),
  },
  handler: async (ctx, args) => {
    let repoAsset = await ctx.db
      .query("assets")
      .withIndex("by_type_value", (q) => q.eq("type", "repository").eq("value", args.repoUrl))
      .first();

    const now = Date.now();

    if (!repoAsset) {
      const assetId = await ctx.db.insert("assets", {
        type: "repository",
        value: args.repoUrl,
        source: "repo-scan-worker",
        confidence: "high",
        status: "active",
        firstSeen: now,
        lastSeen: now,
      });
      repoAsset = (await ctx.db.get(assetId))!;
    }

    const existing = await ctx.db
      .query("secretFindings")
      .withIndex("by_repo", (q) => q.eq("repoAssetId", repoAsset._id))
      .first();

    if (existing) {
      await ctx.db.patch(existing._id, {
        lastSeen: now,
        commitSha: args.commitSha,
        lastScannedSha: args.commitSha,
        verified: args.verified,
      });
      return existing._id;
    }

    return await ctx.db.insert("secretFindings", {
      repoAssetId: repoAsset._id,
      filePath: args.filePath,
      provider: args.provider,
      redactedRef: args.redactedRef,
      commitSha: args.commitSha,
      lastScannedSha: args.commitSha,
      verified: args.verified,
      priority: args.priority,
      status: "open",
      firstSeen: now,
      lastSeen: now,
    });
  },
});
