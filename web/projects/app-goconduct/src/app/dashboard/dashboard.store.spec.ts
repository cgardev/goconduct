import { create } from '@bufbuild/protobuf';
import {
  ComponentSchema,
  RelationshipSchema,
  type Component,
} from 'lib-api-gen/gen/v1/graph_pb';
import { buildGraphLayout, filterComponents } from './dashboard.store';

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
  it('filters by role and text before sorting by afferent coupling', () => {
    const components = [
      component('internal/library/clock', 'library', 2),
      component('internal/library/eventbus', 'library', 7),
      component('internal/module/orders', 'shared-module', 9),
    ];

    const result = filterComponents(components, 'library', 'internal/library');

    expect(result.map((item) => item.id)).toEqual([
      'internal/library/eventbus',
      'internal/library/clock',
    ]);
  });

  it('does not mutate the source collection', () => {
    const components = [component('second', 'library', 1), component('first', 'library', 2)];

    filterComponents(components, 'all', '');

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
