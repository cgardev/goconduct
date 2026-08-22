import { RuntimeEnvironmentSource } from '../app/kernel/runtime-environment';

/**
 * Build-time environment, selected through the file replacements of Angular per
 * build configuration. It decides nothing but the production flag and where the
 * runtime environment is read from. The endpoint value itself lives in the
 * runtime environment, so one production bundle serves several deployments.
 */
export interface AppEnvironment {
  readonly production: boolean;
  readonly runtimeEnvironment: RuntimeEnvironmentSource;
}
