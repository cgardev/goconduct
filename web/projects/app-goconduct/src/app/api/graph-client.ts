import { InjectionToken, Provider } from '@angular/core';
import { createClient, type Client } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { GraphService } from 'lib-api-gen/gen/v1/graph_pb';

/** Browser client for the generated GraphService contract. */
export type GraphClient = Client<typeof GraphService>;

/** Injection token for the same-origin GraphService client. */
export const GRAPH_CLIENT = new InjectionToken<GraphClient>('goconduct.graph-client');

/** Provides the generated Connect client over the browser's current origin. */
export function provideGraphClient(): Provider {
  return {
    provide: GRAPH_CLIENT,
    useFactory: (): GraphClient =>
      createClient(
        GraphService,
        createConnectTransport({
          baseUrl: window.location.origin,
          useBinaryFormat: true,
        }),
      ),
  };
}
