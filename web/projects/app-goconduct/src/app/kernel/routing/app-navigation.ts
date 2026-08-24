import { ROUTE_PATH } from './route-paths';

/** Absolute router command array suitable for `router.navigate` or `routerLink`. */
export type NavCommands = readonly string[];

/**
 * Typed navigation builders. Components and the navigation model build their
 * links from these helpers instead of hand-written command arrays, so the route
 * commands stay in one place as pages are added.
 */
export const navTo = {
  /** Summary of the analysis and the dependency map. */
  overview(): NavCommands {
    return ['/', ROUTE_PATH.overview];
  },

  /** The strategy the repository follows: de facto layers, matrix, and contradictions. */
  strategy(): NavCommands {
    return ['/', ROUTE_PATH.strategy];
  },

  /** Coupling metrics of every analyzed component. */
  components(): NavCommands {
    return ['/', ROUTE_PATH.components];
  },

  /** The balance of every component: zones, ranking, and distribution. */
  balance(): NavCommands {
    return ['/', ROUTE_PATH.balance];
  },

  /** Results of the deterministic architecture rules. */
  findings(): NavCommands {
    return ['/', ROUTE_PATH.findings];
  },

  /** UML-style diagram of the Go types of one component. */
  types(): NavCommands {
    return ['/', ROUTE_PATH.types];
  },
} as const;
