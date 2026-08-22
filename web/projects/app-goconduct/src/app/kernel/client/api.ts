import { InjectionToken, Provider } from '@angular/core';
import { createClient, type Client, type Transport } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { GraphService } from 'lib-api-gen/gen/v1/graph_pb';
import { QualityService } from 'lib-api-gen/gen/v1/quality_pb';

/** Browser client for the generated GraphService contract. */
export type GraphClient = Client<typeof GraphService>;

/** Browser client for the generated QualityService contract. */
export type QualityClient = Client<typeof QualityService>;

/** Dependency injection handle for the GraphService client. */
export const GRAPH_CLIENT = new InjectionToken<GraphClient>('GRAPH_CLIENT');

/** Dependency injection handle for the QualityService client. */
export const QUALITY_CLIENT = new InjectionToken<QualityClient>('QUALITY_CLIENT');

/**
 * Provides every generated Connect client over one transport. The binary
 * Protobuf format keeps the payloads small, and the graph of a large repository
 * is the largest response the console reads.
 *
 * @param baseUrl - Base URL of the `goconduct` server. A relative value such as
 *   `/` targets the origin that serves the console, which is the embedded
 *   deployment.
 */
export function provideApi(baseUrl: string): Provider[] {
  return [
    {
      provide: GRAPH_CLIENT,
      useFactory: (): GraphClient => createClient(GraphService, createTransport(baseUrl)),
    },
    {
      provide: QUALITY_CLIENT,
      useFactory: (): QualityClient => createClient(QualityService, createTransport(baseUrl)),
    },
  ];
}

// createTransport resolves the configured base URL against the current origin. A
// relative value is therefore always resolved against the origin itself, and
// never against the directory of the route the browser reloaded on.
function createTransport(baseUrl: string): Transport {
  return createConnectTransport({
    baseUrl: new URL(baseUrl, window.location.origin).toString(),
    useBinaryFormat: true,
  });
}
