/**
 * Structural route paths used by the root route configuration.
 *
 * The console reads one repository, the one the server analyzes, so no path
 * carries a repository segment. Every path names a reading of that single
 * graph.
 */
export const ROUTE_PATH = {
  /** The empty path, matched by the shell route as a zero-segment prefix. */
  empty: '',
  /** Summary of the analysis and the dependency map. */
  overview: 'overview',
  /** The strategy the repository follows: de facto layers, matrix, and contradictions. */
  strategy: 'strategy',
  /** Coupling metrics of every analyzed component. */
  components: 'components',
  /** The balance of every component: zones, ranking, and distribution. */
  balance: 'balance',
  /** Results of the deterministic architecture rules. */
  findings: 'findings',
  /** UML-style diagram of the Go types of one component. */
  types: 'types',
  /** Catch-all path that renders the in-shell 404 page. */
  wildcard: '**',
} as const;
