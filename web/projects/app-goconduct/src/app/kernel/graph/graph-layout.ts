import type { Component as GraphComponent, Relationship } from 'lib-api-gen/gen/v1/graph_pb';

/** One positioned component in the architecture map. */
export interface GraphNode {
  readonly component: GraphComponent;
  readonly x: number;
  readonly y: number;
}

/** One positioned dependency in the architecture map. */
export interface GraphEdge {
  readonly relationship: Relationship;
  readonly x1: number;
  readonly y1: number;
  readonly x2: number;
  readonly y2: number;
}

/** View-ready architecture map. */
export interface GraphLayout {
  readonly nodes: readonly GraphNode[];
  readonly edges: readonly GraphEdge[];
}

/** Column the components table is sorted by. */
export type ComponentSortColumn =
  | 'name'
  | 'role'
  | 'afferentCoupling'
  | 'efferentCoupling'
  | 'instability'
  | 'mainSequenceDistance';

/** Direction one column is sorted in. */
export type SortDirection = 'ascending' | 'descending';

/** How the components table is sorted. */
export interface ComponentSort {
  readonly column: ComponentSortColumn;
  readonly direction: SortDirection;
}

/** Value of the role filter that keeps every role. */
export const EVERY_ROLE = 'all';

/** How many components the map places, ordered by total coupling. */
export const GRAPH_LAYOUT_LIMIT = 18;

/**
 * Sort the table starts on: the most depended-upon component first.
 *
 * A reader opens this table to find what the repository leans on, so the first
 * row answers that question without a single interaction.
 */
export const DEFAULT_COMPONENT_SORT: ComponentSort = {
  column: 'afferentCoupling',
  direction: 'descending',
};

/** Filters components by role and a case-insensitive text query. */
export function filterComponents(
  components: readonly GraphComponent[],
  role: string,
  query: string,
): GraphComponent[] {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  return components
    .filter((component) => role === EVERY_ROLE || component.role === role)
    .filter((component) => {
      if (normalizedQuery === '') {
        return true;
      }
      return [component.id, component.name, component.category, component.application]
        .join(' ')
        .toLocaleLowerCase()
        .includes(normalizedQuery);
    });
}

/**
 * Sorts components by one column.
 *
 * The identifier breaks every tie, so the order is total. Two runs over the
 * same graph therefore produce the same table, which a deterministic analysis
 * has to guarantee.
 */
export function sortComponents(
  components: readonly GraphComponent[],
  sort: ComponentSort,
): GraphComponent[] {
  const sign = sort.direction === 'ascending' ? 1 : -1;
  return [...components].sort(
    (first, second) => sign * compare(first, second, sort.column) || first.id.localeCompare(second.id),
  );
}

// compare orders two components by one column. A text column compares by
// locale, and a numeric column by value.
function compare(
  first: GraphComponent,
  second: GraphComponent,
  column: ComponentSortColumn,
): number {
  if (column === 'name') {
    return first.name.localeCompare(second.name);
  }
  if (column === 'role') {
    return first.role.localeCompare(second.role);
  }
  return first[column] - second[column];
}

/** Builds a stable circular layout for the most connected components. */
export function buildGraphLayout(
  components: readonly GraphComponent[],
  relationships: readonly Relationship[],
  limit = GRAPH_LAYOUT_LIMIT,
): GraphLayout {
  const selected = [...components]
    .sort(
      (first, second) =>
        second.afferentCoupling + second.efferentCoupling -
          (first.afferentCoupling + first.efferentCoupling) ||
        first.id.localeCompare(second.id),
    )
    .slice(0, limit);
  // A small ring needs a shorter radius, or its labels reach the panel border.
  const radius = selected.length < 8 ? 33 : 39;
  const nodes = selected.map((component, index): GraphNode => {
    const angle = (Math.PI * 2 * index) / Math.max(selected.length, 1) - Math.PI / 2;
    return {
      component,
      x: 50 + Math.cos(angle) * radius,
      y: 50 + Math.sin(angle) * radius,
    };
  });
  const positions = new Map(nodes.map((node) => [node.component.id, node]));
  const edges = relationships.flatMap((relationship): GraphEdge[] => {
    const source = positions.get(relationship.source);
    const target = positions.get(relationship.target);
    if (source === undefined || target === undefined) {
      return [];
    }
    return [
      {
        relationship,
        x1: source.x,
        y1: source.y,
        x2: target.x,
        y2: target.y,
      },
    ];
  });
  return { nodes, edges };
}
