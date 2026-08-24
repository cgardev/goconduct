import { create } from '@bufbuild/protobuf';
import { ComponentSchema, type Component } from 'lib-api-gen/gen/v1/graph_pb';
import { abstractionLabel, readMetrics, stabilityLabel } from './metric-reading';

function component(parts: Partial<Omit<Component, '$typeName' | '$unknown'>>): Component {
  return create(ComponentSchema, { id: 'pkg/example', name: 'example', ...parts });
}

describe('readMetrics', () => {
  it('names a component nothing touches as isolated', () => {
    const reading = readMetrics(component({ afferentCoupling: 0, efferentCoupling: 0 }));

    expect(reading.zone).toBe('isolated');
  });

  it('names an application root as free to change', () => {
    const reading = readMetrics(
      component({ afferentCoupling: 0, efferentCoupling: 12, instability: 1 }),
    );

    expect(reading.zone).toBe('free');
    expect(reading.explanation).toContain('12 components');
  });

  it('places a stable concrete component in the zone of pain', () => {
    const reading = readMetrics(
      component({
        afferentCoupling: 17,
        efferentCoupling: 0,
        instability: 0,
        abstractness: 0,
        mainSequenceDistance: 1,
      }),
    );

    expect(reading.zone).toBe('pain');
    expect(reading.explanation).toContain('17 components depend on it');
  });

  it('places an unstable abstract component in the zone of uselessness', () => {
    const reading = readMetrics(
      component({
        afferentCoupling: 1,
        efferentCoupling: 9,
        instability: 0.9,
        abstractness: 0.9,
        mainSequenceDistance: 0.8,
      }),
    );

    expect(reading.zone).toBe('useless');
  });

  it('reads a component near the diagonal as balanced', () => {
    const reading = readMetrics(
      component({
        afferentCoupling: 1,
        efferentCoupling: 9,
        instability: 0.9,
        abstractness: 0.16,
        mainSequenceDistance: 0.06,
      }),
    );

    expect(reading.zone).toBe('balanced');
    expect(reading.explanation).toContain('1 component depends');
  });

  it('reads a component far from the diagonal as off balance', () => {
    const reading = readMetrics(
      component({
        afferentCoupling: 3,
        efferentCoupling: 1,
        instability: 0.25,
        abstractness: 0.3,
        mainSequenceDistance: 0.45,
      }),
    );

    expect(reading.zone).toBe('drifting');
  });
});

describe('stabilityLabel', () => {
  it('names the four bands of instability', () => {
    expect(stabilityLabel(0)).toBe('Stable');
    expect(stabilityLabel(0.3)).toBe('Mostly stable');
    expect(stabilityLabel(0.7)).toBe('Mostly unstable');
    expect(stabilityLabel(1)).toBe('Unstable');
  });
});

describe('abstractionLabel', () => {
  it('names the four bands of abstractness', () => {
    expect(abstractionLabel(0)).toBe('Concrete');
    expect(abstractionLabel(0.3)).toBe('Mostly concrete');
    expect(abstractionLabel(0.7)).toBe('Mostly abstract');
    expect(abstractionLabel(1)).toBe('Abstract');
  });
});
