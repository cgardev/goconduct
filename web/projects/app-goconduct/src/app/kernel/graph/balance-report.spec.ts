import { create } from '@bufbuild/protobuf';
import { ComponentSchema, type Component as GraphComponent } from 'lib-api-gen/gen/v1/graph_pb';
import { bucketDistances, rankOffenders, summarizeZones } from './balance-report';

function component(parts: {
  id: string;
  name?: string;
  afferentCoupling?: number;
  efferentCoupling?: number;
  instability?: number;
  abstractness?: number;
  mainSequenceDistance?: number;
}): GraphComponent {
  return create(ComponentSchema, {
    name: parts.id.split('/').at(-1) ?? parts.id,
    role: 'library',
    ...parts,
  });
}

const PAINFUL = component({
  id: 'pkg/failure',
  afferentCoupling: 17,
  instability: 0,
  abstractness: 0,
  mainSequenceDistance: 1,
});
const BALANCED = component({
  id: 'pkg/report',
  afferentCoupling: 2,
  efferentCoupling: 2,
  instability: 0.5,
  abstractness: 0.4,
  mainSequenceDistance: 0.1,
});
const FREE = component({
  id: 'cmd/goconduct',
  efferentCoupling: 12,
  instability: 1,
  mainSequenceDistance: 0,
});
const ISOLATED = component({ id: 'pkg/orphan', mainSequenceDistance: 1 });

describe('summarizeZones', () => {
  it('counts every zone and keeps the fixed order', () => {
    const zones = summarizeZones([PAINFUL, BALANCED, FREE, ISOLATED]);

    expect(zones.map((zone) => zone.zone)).toEqual([
      'pain',
      'useless',
      'drifting',
      'balanced',
      'free',
      'isolated',
    ]);
    expect(zones.find((zone) => zone.zone === 'pain')?.count).toBe(1);
    expect(zones.find((zone) => zone.zone === 'balanced')?.count).toBe(1);
    expect(zones.find((zone) => zone.zone === 'useless')?.count).toBe(0);
  });
});

describe('rankOffenders', () => {
  it('ranks only the zones that call for attention, the riskiest first', () => {
    const drifting = component({
      id: 'pkg/drifting',
      afferentCoupling: 4,
      instability: 0.2,
      abstractness: 0.2,
      mainSequenceDistance: 0.6,
    });

    const ranking = rankOffenders([BALANCED, drifting, PAINFUL, FREE, ISOLATED]);

    expect(ranking.map((entry) => entry.id)).toEqual(['pkg/failure', 'pkg/drifting']);
    expect(ranking[0]?.summary).toContain('Stable and concrete');
    expect(ranking[0]?.summary).toContain('17 components depend on it');
    expect(ranking[0]?.dependents).toBe(17);
  });

  it('weighs the distance by the number of dependents', () => {
    const nearButLoadBearing = component({
      id: 'pkg/load-bearing',
      afferentCoupling: 30,
      instability: 0.1,
      abstractness: 0.15,
      mainSequenceDistance: 0.75,
    });

    const ranking = rankOffenders([PAINFUL, nearButLoadBearing]);

    expect(ranking.map((entry) => entry.id)).toEqual(['pkg/load-bearing', 'pkg/failure']);
  });

  it('labels an entry by its identifier when two entries share a name', () => {
    const twin = component({
      id: 'internal/failure',
      afferentCoupling: 9,
      instability: 0,
      abstractness: 0,
      mainSequenceDistance: 0.9,
    });

    const ranking = rankOffenders([PAINFUL, twin]);

    expect(ranking.map((entry) => entry.label)).toEqual(['pkg/failure', 'internal/failure']);
  });

  it('cuts the ranking at the given limit', () => {
    const many = Array.from({ length: 4 }, (_, index) =>
      component({
        id: `pkg/c${index}`,
        afferentCoupling: 3,
        instability: 0,
        abstractness: 0,
        mainSequenceDistance: 0.9 - index / 10,
      }),
    );

    expect(rankOffenders(many, 2)).toHaveLength(2);
  });
});

describe('bucketDistances', () => {
  it('buckets only the components whose balance carries meaning', () => {
    const buckets = bucketDistances([PAINFUL, BALANCED, FREE, ISOLATED]);

    expect(buckets).toHaveLength(10);
    expect(buckets.reduce((total, bucket) => total + bucket.count, 0)).toBe(2);
  });

  it('closes the last bucket so a distance of one is counted', () => {
    const buckets = bucketDistances([PAINFUL]);

    expect(buckets.at(-1)?.count).toBe(1);
  });
});
