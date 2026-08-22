import { InjectionToken } from '@angular/core';

/**
 * Configuration resolved at startup, before the application is bootstrapped, so
 * no endpoint URL is compiled into the bundle. One production build therefore
 * serves every deployment, each with its own runtime environment document.
 */
export interface RuntimeEnvironment {
  /**
   * Base URL the Connect RPC transport prepends to every request. The console
   * ships inside the `goconduct` binary, which serves both the assets and the
   * services, so `/` is the normal value. An absolute URL targets a server on
   * another origin.
   */
  apiBaseUrl: string;
}

/**
 * How the {@link RuntimeEnvironment} is obtained at startup: either fetched from
 * a JSON document, which the deployment rewrites, or provided inline by a build
 * configuration, for local development.
 */
export type RuntimeEnvironmentSource =
  | { readonly kind: 'file'; readonly path: string }
  | { readonly kind: 'inline'; readonly values: RuntimeEnvironment };

/** Dependency injection handle for the resolved {@link RuntimeEnvironment}. */
export const RUNTIME_ENVIRONMENT = new InjectionToken<RuntimeEnvironment>('RUNTIME_ENVIRONMENT');

/**
 * Raised when the runtime environment cannot be read, is not valid JSON, or does
 * not carry every field the application requires. The message always names the
 * offending field, so a misconfigured deployment is diagnosable from the browser
 * console alone.
 */
export class RuntimeEnvironmentError extends Error {
  constructor(message: string, options?: ErrorOptions) {
    super(message, options);
    this.name = 'RuntimeEnvironmentError';
  }
}

/**
 * Resolves the runtime environment from its configured source and validates it
 * in full. A file source is fetched once with caching disabled, so the
 * deployment can rewrite the document without a rebuild.
 *
 * @param source - Where to read the runtime environment from.
 * @returns The validated runtime environment.
 * @throws RuntimeEnvironmentError When the document is missing, unreadable, or
 *   incomplete. The error names the field that failed.
 */
export async function loadRuntimeEnvironment(
  source: RuntimeEnvironmentSource,
): Promise<RuntimeEnvironment> {
  if (source.kind === 'inline') {
    return validateRuntimeEnvironment(source.values, 'the inline runtime environment');
  }

  const environmentDocument = await fetchRuntimeEnvironmentDocument(source.path);
  return validateRuntimeEnvironment(environmentDocument, source.path);
}

// fetchRuntimeEnvironmentDocument retrieves the raw JSON document. It translates
// every transport failure and parsing failure into a RuntimeEnvironmentError, so
// callers handle one error type only.
async function fetchRuntimeEnvironmentDocument(path: string): Promise<unknown> {
  let response: Response;
  try {
    response = await fetch(path, { cache: 'no-store' });
  } catch (cause: unknown) {
    throw new RuntimeEnvironmentError(`Failed to request the runtime environment from ${path}`, {
      cause,
    });
  }

  if (!response.ok) {
    throw new RuntimeEnvironmentError(
      `Failed to load the runtime environment from ${path}: ` +
        `${response.status} ${response.statusText}`,
    );
  }

  try {
    return (await response.json()) as unknown;
  } catch (cause: unknown) {
    throw new RuntimeEnvironmentError(`The runtime environment at ${path} is not valid JSON`, {
      cause,
    });
  }
}

// validateRuntimeEnvironment rebuilds the runtime environment field by field. An
// unknown document therefore never reaches the application as a partly populated
// value, and every rejection names exactly one field.
function validateRuntimeEnvironment(
  environmentDocument: unknown,
  origin: string,
): RuntimeEnvironment {
  if (!isRecord(environmentDocument)) {
    throw new RuntimeEnvironmentError(`The runtime environment from ${origin} is not a JSON object`);
  }

  return {
    apiBaseUrl: requireNonEmptyString(environmentDocument['apiBaseUrl'], 'apiBaseUrl', origin),
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function requireNonEmptyString(value: unknown, field: string, origin: string): string {
  if (typeof value !== 'string' || value.trim() === '') {
    throw new RuntimeEnvironmentError(
      `The runtime environment from ${origin} requires "${field}" to be a non-empty string`,
    );
  }
  return value;
}
