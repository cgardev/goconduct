import { fakeComponent, fakeRelationship } from '../../testing/fake-clients';
import { buildStrategyReport, groupOf } from './strategy-report';

const COMPONENTS = [
  fakeComponent('cmd/goconduct', 'application'),
  fakeComponent('internal/module/orders', 'application-module'),
  fakeComponent('internal/module/billing', 'application-module'),
  fakeComponent('internal/library/telemetry', 'library'),
  fakeComponent('internal/library/clock', 'library'),
];

const RELATIONSHIPS = [
  fakeRelationship('cmd/goconduct', 'internal/module/orders'),
  fakeRelationship('cmd/goconduct', 'internal/module/billing'),
  fakeRelationship('internal/module/orders', 'internal/library/telemetry'),
  fakeRelationship('internal/module/orders', 'internal/library/clock'),
  fakeRelationship('internal/module/orders', 'internal/module/billing'),
  fakeRelationship('internal/library/telemetry', 'internal/library/clock'),
];

describe('groupOf', () => {
  it('names the group after the first two path segments', () => {
    expect(groupOf('cmd/cloudcell/internal/module/license')).toBe('cmd/cloudcell');
    expect(groupOf('internal/library/telemetry')).toBe('internal/library');
    expect(groupOf('pkg')).toBe('pkg');
  });
});

describe('buildStrategyReport', () => {
  it('aggregates the components and their dependencies into groups', () => {
    const report = buildStrategyReport(COMPONENTS, RELATIONSHIPS);

    expect(report.groups.map((group) => group.id)).toEqual([
      'cmd/goconduct',
      'internal/library',
      'internal/module',
    ]);
    expect(report.groups.find((group) => group.id === 'internal/module')?.components).toBe(2);
    const moduleToLibrary = report.edges.find(
      (edge) => edge.source === 'internal/module' && edge.target === 'internal/library',
    );
    expect(moduleToLibrary?.weight).toBe(2);
  });

  it('keeps a dependency inside one group out of the edges', () => {
    const report = buildStrategyReport(COMPONENTS, RELATIONSHIPS);

    expect(
      report.edges.some(
        (edge) => edge.source === 'internal/module' && edge.target === 'internal/module',
      ),
    ).toBe(false);
  });

  it('keeps a test-only dependency out of the strategy', () => {
    const report = buildStrategyReport(COMPONENTS, [
      fakeRelationship('internal/library/clock', 'internal/module/orders', true),
    ]);

    expect(report.edges).toHaveLength(0);
  });

  /**
   * The layers come from the dependencies themselves: a group sits one layer
   * above the highest group it depends on, so the foundation sits at zero.
   */
  it('orders the groups into de facto layers', () => {
    const report = buildStrategyReport(COMPONENTS, RELATIONSHIPS);

    expect(report.layers).toEqual([
      ['internal/library'],
      ['internal/module'],
      ['cmd/goconduct'],
    ]);
    expect(report.groups.every((group) => !group.inCycle)).toBe(true);
  });

  it('marks two groups that depend on each other as one cyclic unit', () => {
    const report = buildStrategyReport(COMPONENTS, [
      ...RELATIONSHIPS,
      fakeRelationship('internal/library/clock', 'internal/module/orders'),
    ]);

    const library = report.groups.find((group) => group.id === 'internal/library');
    const module = report.groups.find((group) => group.id === 'internal/module');
    expect(library?.inCycle).toBe(true);
    expect(module?.inCycle).toBe(true);
    expect(library?.layer).toBe(module?.layer);
    const upward = report.edges.find(
      (edge) => edge.source === 'internal/library' && edge.target === 'internal/module',
    );
    expect(upward?.cyclic).toBe(true);
  });

  it('produces the same report for the same input', () => {
    const first = buildStrategyReport(COMPONENTS, RELATIONSHIPS);
    const second = buildStrategyReport([...COMPONENTS].reverse(), [...RELATIONSHIPS].reverse());

    expect(first).toEqual(second);
  });
});
