import { create } from '@bufbuild/protobuf';
import { ComponentSchema, RelationshipSchema, type Component } from 'lib-api-gen/gen/v1/graph_pb';
import {
  buildGraphLayout,
  DEFAULT_COMPONENT_SORT,
  EVERY_ROLE,
  filterComponents,
  sortComponents,
} from './graph-layout';

function component(
  id: string,
  role: string,
  afferentCoupling: number,
  efferentCoupling = 0,
): Component {
  return create(ComponentSchema, {
    id,
    name: id.split('/').at(-1) ?? id,
    role,
    afferentCoupling,
    efferentCoupling,
  });
}

describe('filterComponents', () => {
  it('keeps only the components that match both the role and the text', () => {
    const components = [
      component('internal/library/clock', 'library', 2),
      component('internal/library/eventbus', 'library', 7),
      component('internal/module/orders', 'shared-module', 9),
    ];

    const result = filterComponents(components, 'library', 'internal/library');

    expect(result.map((item) => item.id)).toEqual([
      'internal/library/clock',
      'internal/library/eventbus',
    ]);
  });

  it('keeps every role under the catch-all value', () => {
    const components = [component('a', 'library', 1), component('b', 'application', 1)];

    expect(filterComponents(components, EVERY_ROLE, '')).toHaveLength(2);
  });

  it('ignores the case and the surrounding blanks of the query', () => {
    const components = [component('internal/library/Clock', 'library', 1)];

    expect(filterComponents(components, EVERY_ROLE, '  CLOCK  ')).toHaveLength(1);
  });

  it('does not mutate the source collection', () => {
    const components = [component('second', 'library', 1), component('first', 'library', 2)];

    filterComponents(components, EVERY_ROLE, '');

    expect(components.map((item) => item.id)).toEqual(['second', 'first']);
  });
});

describe('sortComponents', () => {
  it('orders a numeric column by value in both directions', () => {
    const components = [
      component('low', 'library', 1),
      component('high', 'library', 9),
      component('middle', 'library', 5),
    ];

    expect(sortComponents(components, DEFAULT_COMPONENT_SORT).map((item) => item.id)).toEqual([
      'high',
      'middle',
      'low',
    ]);
    expect(
      sortComponents(components, { column: 'afferentCoupling', direction: 'ascending' }).map(
        (item) => item.id,
      ),
    ).toEqual(['low', 'middle', 'high']);
  });

  it('orders a text column by locale', () => {
    const components = [component('b', 'library', 1), component('a', 'application', 1)];

    expect(
      sortComponents(components, { column: 'role', direction: 'ascending' }).map(
        (item) => item.role,
      ),
    ).toEqual(['application', 'library']);
  });

  /**
   * The analysis is deterministic, so the table it feeds has to be too. Without
   * a tie break, two components with the same value could swap rows between two
   * renders of the same graph.
   */
  it('breaks a tie by identifier so the order is total', () => {
    const components = [
      component('zulu', 'library', 4),
      component('alpha', 'library', 4),
      component('mike', 'library', 4),
    ];

    expect(sortComponents(components, DEFAULT_COMPONENT_SORT).map((item) => item.id)).toEqual([
      'alpha',
      'mike',
      'zulu',
    ]);
  });

  it('does not mutate the source collection', () => {
    const components = [component('second', 'library', 1), component('first', 'library', 2)];

    sortComponents(components, DEFAULT_COMPONENT_SORT);

    expect(components.map((item) => item.id)).toEqual(['second', 'first']);
  });
});

describe('buildGraphLayout', () => {
  it('keeps only relationships whose endpoints are visible', () => {
    const components = [
      component('application', 'application', 4, 1),
      component('module', 'application-module', 2, 3),
      component('hidden', 'library', 0, 0),
    ];
    const relationships = [
      create(RelationshipSchema, { source: 'application', target: 'module' }),
      create(RelationshipSchema, { source: 'module', target: 'hidden' }),
    ];

    const result = buildGraphLayout(components, relationships, 2);

    expect(result.nodes.map((node) => node.component.id)).toEqual(['application', 'module']);
    expect(result.edges).toHaveLength(1);
    expect(result.edges[0]?.relationship.target).toBe('module');
  });

  it('returns finite positions for a single component', () => {
    const result = buildGraphLayout([component('only', 'library', 0)], []);

    expect(result.nodes).toHaveLength(1);
    expect(Number.isFinite(result.nodes[0]?.x)).toBe(true);
    expect(Number.isFinite(result.nodes[0]?.y)).toBe(true);
  });
});
