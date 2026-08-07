import { HttpTransport } from './http.transport';
import { TrawlTransport } from './trawl-transport';
import { WailsTransport } from './wails.transport';

export * from './trawl-transport';
export * from './wails.transport';
export * from './http.transport';

/**
 * Chooses the transport for the environment the bundle finds itself in.
 *
 * Detection rather than a build flag, because it is the same bundle: the
 * desktop binary embeds these exact files, and the container serves them from
 * nginx. A compile-time switch would mean two builds to keep in step, and the
 * one nobody tests is the one that breaks.
 *
 * The Wails runtime is authoritative when present. Its absence is not an error
 * — it is simply the browser deployment, which reaches the same application
 * layer over HTTP.
 */
export function createTransport(): TrawlTransport {
  return WailsTransport.available() ? new WailsTransport() : new HttpTransport();
}
