import { AppEnvironment } from './interface';

/**
 * Production environment. The runtime environment is fetched from a document
 * the deployment rewrites with the real endpoint value when the container
 * starts, so the same bundle serves every deployment. The `goconduct` binary
 * embeds the document with the rest of the assets.
 */
export const environment: AppEnvironment = {
  production: true,
  runtimeEnvironment: { kind: 'file', path: '/runtime-env/runtime-env.json' },
};
