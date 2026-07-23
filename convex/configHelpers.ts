import { internalQuery } from "./_generated/server";

export const getInternalConfig = internalQuery({
  args: {},
  handler: async (ctx) => {
    return await ctx.db.query("config").first();
  },
});
