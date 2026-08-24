import type { Component as GraphComponent } from 'lib-api-gen/gen/v1/graph_pb';

/**
 * Region of the balance a component sits in.
 *
 * The regions follow Robert C. Martin's package metrics: a component balances
 * stability against abstraction, and the chart names the two corners where
 * that balance is lost.
 */
export type BalanceZone = 'balanced' | 'drifting' | 'pain' | 'useless' | 'isolated' | 'free';

/** Plain-language reading of one component's coupling metrics. */
export interface MetricReading {
  readonly zone: BalanceZone;
  /** Two or three words that name the situation. */
  readonly headline: string;
  /** What the numbers say about this component, in one or two sentences. */
  readonly explanation: string;
  /** What a reader does with that information. */
  readonly advice: string;
}

/**
 * Limits of the corner zones.
 *
 * They match the `stableLowAbstraction` policy the server applies, so the chart
 * and the findings list agree on which components sit in the zone of pain.
 */
export const ZONE_LIMIT = 0.2;

/** Largest distance from the main sequence a component can have and still count as balanced. */
export const BALANCED_DISTANCE = 0.3;

/** Reads the coupling metrics of one component into plain language. */
export function readMetrics(component: GraphComponent): MetricReading {
  const importers = component.afferentCoupling;
  const dependencies = component.efferentCoupling;

  if (importers === 0 && dependencies === 0) {
    return {
      zone: 'isolated',
      headline: 'Isolated',
      explanation: 'Nothing depends on this component, and it depends on nothing.',
      advice: 'Its metrics carry no information. Check whether the component is still used.',
    };
  }

  if (importers === 0) {
    return {
      zone: 'free',
      headline: 'Free to change',
      explanation:
        `Nothing depends on this component, while it depends on ${count(dependencies, 'component')}. ` +
        'A change here breaks nothing else.',
      advice: 'This is the expected shape of an application entry point. No action is needed.',
    };
  }

  if (component.instability <= ZONE_LIMIT && component.abstractness <= ZONE_LIMIT) {
    return {
      zone: 'pain',
      headline: 'Stable and concrete',
      explanation:
        `${dependents(importers)} and it offers almost no interface. ` +
        'A change here spreads to every one of them, and nothing lets them extend it instead.',
      advice:
        'This is fine for a data model, a set of constants or generated code. ' +
        'It is a risk when the component holds logic that changes often.',
    };
  }

  if (component.instability >= 1 - ZONE_LIMIT && component.abstractness >= 1 - ZONE_LIMIT) {
    return {
      zone: 'useless',
      headline: 'Abstract but unused',
      explanation:
        'The component is mostly interfaces, yet almost nothing depends on it. ' +
        'The abstraction protects nobody.',
      advice: 'Remove the unused interfaces or move them next to the code that implements them.',
    };
  }

  if (component.mainSequenceDistance <= BALANCED_DISTANCE) {
    return {
      zone: 'balanced',
      headline: 'Balanced',
      explanation:
        `${dependents(importers)} and it depends on ${count(dependencies, 'component')}. ` +
        'Its amount of interface matches how much the rest of the repository leans on it.',
      advice: 'No action is needed.',
    };
  }

  return {
    zone: 'drifting',
    headline: 'Off balance',
    explanation:
      `${dependents(importers)} and it depends on ${count(dependencies, 'component')}. ` +
      'It sits away from the diagonal: its amount of interface does not match its stability.',
    advice:
      'A stable component gains an interface. An unstable component with many interfaces loses them.',
  };
}

/** Names the instability of a component for a reader who does not know the formula. */
export function stabilityLabel(instability: number): string {
  if (instability <= ZONE_LIMIT) {
    return 'Stable';
  }
  if (instability < 0.5) {
    return 'Mostly stable';
  }
  if (instability < 1 - ZONE_LIMIT) {
    return 'Mostly unstable';
  }
  return 'Unstable';
}

/** Names the abstractness of a component for a reader who does not know the formula. */
export function abstractionLabel(abstractness: number): string {
  if (abstractness <= ZONE_LIMIT) {
    return 'Concrete';
  }
  if (abstractness < 0.5) {
    return 'Mostly concrete';
  }
  if (abstractness < 1 - ZONE_LIMIT) {
    return 'Mostly abstract';
  }
  return 'Abstract';
}

function dependents(importers: number): string {
  return importers === 1 ? '1 component depends on it' : `${importers} components depend on it`;
}

function count(value: number, noun: string): string {
  return value === 1 ? `1 ${noun}` : `${value} ${noun}s`;
}
