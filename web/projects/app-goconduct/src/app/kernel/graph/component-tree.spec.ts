import { ancestorPaths, buildComponentTree } from './component-tree';

const IDENTIFIERS = [
  'cmd/goconduct',
  'internal/library/telemetry',
  'internal/library/telemetry/writers',
  'internal/module/orders',
  'pkg/report',
];

describe('buildComponentTree', () => {
  it('nests every component under its path segments', () => {
    const roots = buildComponentTree(IDENTIFIERS);

    expect(roots.map((root) => root.label)).toEqual(['internal', 'cmd/goconduct', 'pkg/report']);
    const internal = roots[0];
    expect(internal?.children.map((child) => child.label)).toEqual([
      'library/telemetry',
      'module/orders',
    ]);
  });

  /**
   * A chain of directories with one child compacts into one node, the way
   * code editors compact folders, so the tree stays shallow.
   */
  it('compacts a directory chain with a single child', () => {
    const roots = buildComponentTree(['cmd/goconduct']);

    expect(roots).toHaveLength(1);
    expect(roots[0]?.label).toBe('cmd/goconduct');
    expect(roots[0]?.componentId).toBe('cmd/goconduct');
  });

  it('keeps a node selectable when it is a component with nested components', () => {
    const roots = buildComponentTree(IDENTIFIERS);

    const telemetry = roots[0]?.children[0];
    expect(telemetry?.componentId).toBe('internal/library/telemetry');
    expect(telemetry?.children.map((child) => child.componentId)).toEqual([
      'internal/library/telemetry/writers',
    ]);
  });

  it('orders the nodes that can open before the leaves', () => {
    const roots = buildComponentTree(['pkg/zeta', 'pkg/alpha/inner', 'pkg/alpha']);

    const pkg = roots[0];
    expect(pkg?.label).toBe('pkg');
    expect(pkg?.children.map((child) => child.label)).toEqual(['alpha', 'zeta']);
    expect(pkg?.children[0]?.children[0]?.componentId).toBe('pkg/alpha/inner');
  });

  it('produces the same tree for the same input', () => {
    expect(buildComponentTree(IDENTIFIERS)).toEqual(buildComponentTree([...IDENTIFIERS].reverse()));
  });
});

describe('ancestorPaths', () => {
  it('returns the paths that open the tree down to one component', () => {
    const roots = buildComponentTree(IDENTIFIERS);

    expect(ancestorPaths(roots, 'internal/library/telemetry/writers')).toEqual([
      'internal',
      'internal/library/telemetry',
    ]);
  });

  it('returns no paths for an unknown component', () => {
    expect(ancestorPaths(buildComponentTree(IDENTIFIERS), 'missing')).toEqual([]);
  });
});
