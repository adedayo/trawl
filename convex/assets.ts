import { query, mutation } from "./_generated/server";
import { v } from "convex/values";

/**
 * Convex Asset Inventory Queries & Mutations
 */

// Query: List all assets
export const listAssets = query({
  args: {
    status: v.optional(v.union(v.literal("active"), v.literal("pending"), v.literal("inactive"), v.literal("rejected"))),
  },
  handler: async (ctx, args) => {
    if (args.status) {
      return await ctx.db
        .query("assets")
        .withIndex("by_status", (q) => q.eq("status", args.status!))
        .collect();
    }
    return await ctx.db.query("assets").collect();
  },
});

// Mutation: Approve a pending asset for scanning
export const approveAsset = mutation({
  args: {
    assetId: v.id("assets"),
  },
  handler: async (ctx, args) => {
    const asset = await ctx.db.get(args.assetId);
    if (!asset) {
      throw new Error(`Asset not found: ${args.assetId}`);
    }
    await ctx.db.patch(args.assetId, {
      status: "active",
      lastSeen: Date.now(),
    });
    return { success: true };
  },
});

// Mutation: Reject a candidate asset from OSINT review queue
export const rejectAsset = mutation({
  args: {
    assetId: v.id("assets"),
  },
  handler: async (ctx, args) => {
    const asset = await ctx.db.get(args.assetId);
    if (!asset) {
      throw new Error(`Asset not found: ${args.assetId}`);
    }
    await ctx.db.patch(args.assetId, {
      status: "rejected",
    });
    return { success: true };
  },
});

// Mutation: Insert or update discovered asset
export const upsertDiscoveredAsset = mutation({
  args: {
    type: v.union(v.literal("ip"), v.literal("domain"), v.literal("repository")),
    value: v.string(),
    source: v.string(),
    confidence: v.union(v.literal("high"), v.literal("medium"), v.literal("low")),
  },
  handler: async (ctx, args) => {
    const existing = await ctx.db
      .query("assets")
      .withIndex("by_type_value", (q) => q.eq("type", args.type).eq("value", args.value))
      .first();

    const now = Date.now();

    if (existing) {
      await ctx.db.patch(existing._id, {
        lastSeen: now,
      });
      return existing._id;
    }

    // Auto-promote high confidence assets to active, queue medium/low for review
    const initialStatus = args.confidence === "high" ? "active" : "pending";

    const id = await ctx.db.insert("assets", {
      type: args.type,
      value: args.value,
      source: args.source,
      confidence: args.confidence,
      status: initialStatus,
      firstSeen: now,
      lastSeen: now,
    });

    return id;
  },
});
