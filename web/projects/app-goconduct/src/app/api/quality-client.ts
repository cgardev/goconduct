import { InjectionToken, Provider } from '@angular/core';
import { createClient, type Client } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { QualityService } from 'lib-api-gen/gen/v1/quality_pb';

/** Browser client for the generated QualityService contract. */
export type QualityClient = Client<typeof QualityService>;

/** Injection token for the same-origin QualityService client. */
export const QUALITY_CLIENT = new InjectionToken<QualityClient>('goconduct.quality-client');

/** Provides the generated Connect quality client. */
export function provideQualityClient(): Provider {
  return {
    provide: QUALITY_CLIENT,
    useFactory: (): QualityClient =>
      createClient(
        QualityService,
        createConnectTransport({
          baseUrl: window.location.origin,
          useBinaryFormat: true,
        }),
      ),
  };
}
