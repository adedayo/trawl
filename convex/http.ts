import { httpRouter } from "convex/server";
import { httpAction } from "./_generated/server";
import { internal } from "./_generated/api";

/**
 * Convex HTTP Router — Ingestion & API Webhooks
 */
const http = httpRouter();

// HTTP Action: Ingest Scan Worker Payload (POST /api/ingest/scan)
http.route({
  path: "/api/ingest/scan",
  method: "POST",
  handler: httpAction(async (ctx, request) => {
    try {
      const body = await request.json();

      // Enforce Scope Authorization Gate: reject ingestion if scope authorization is unsigned
      const configDoc = await ctx.runQuery(internal.configHelpers.getInternalConfig);
      if (configDoc && !configDoc.authorizationSignedAt) {
        return new Response(
          JSON.stringify({
            error: "UNAUTHORIZED_SCOPE",
            message: "Target scope authorization must be digitally signed in the Trawl UI before scan ingestion is allowed.",
          }),
          { status: 403, headers: { "Content-Type": "application/json" } }
        );
      }

      // Invoke ingestion mutation
      const result = await ctx.runMutation(internal.findings.ingestScanResults, {
        jobRunId: body.jobRunId || `scan-${Date.now()}`,
        naabu: body.naabu,
        httpx: body.httpx,
        nuclei: body.nuclei,
      });

      return new Response(JSON.stringify(result), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    } catch (err: any) {
      return new Response(
        JSON.stringify({ error: "INGESTION_FAILED", message: err.message || String(err) }),
        { status: 500, headers: { "Content-Type": "application/json" } }
      );
    }
  }),
});

export default http;
