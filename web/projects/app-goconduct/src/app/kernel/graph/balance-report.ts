import type { Component as GraphComponent } from 'lib-api-gen/gen/v1/graph_pb';
import { DIAGRAM_COLORS } from '../diagram/diagram-colors';
import { BALANCED_DISTANCE, readMetrics, type BalanceZone } from './metric-reading';

/** How many components the balance ranking shows. */
export const RANKING_LIMIT = 15;

/** Width of one bucket of the distance distribution. */
const BUCKET_WIDTH = 0.1;

/** Fixed presentation order of the zones: the problems first, the quiet states last. */
export const ZONE_ORDER: readonly BalanceZone[] = [
  'pain',
  'useless',
  'drifting',
  'balanced',
  'free',
  'isolated',
];

/** Title and one-line meaning of each zone, shared by the balance page and the overview. */
export const ZONE_DETAILS: Readonly<Record<BalanceZone, { title: string; meaning: string }>> = {
  pain: { title: 'Zone of pain', meaning: 'Many depend on it and it offers no interface.' },
  useless: { title: 'Zone of uselessness', meaning: 'Mostly interfaces that almost nothing uses.' },
  drifting: { title: 'Off balance', meaning: 'The amount of interface does not match the stability.' },
  balanced: { title: 'Balanced', meaning: 'The interface matches how much the repository leans on it.' },
  free: { title: 'Free to change', meaning: 'Nothing depends on it, so a change breaks nothing.' },
  isolated: { title: 'Isolated', meaning: 'No dependency in either direction.' },
};

/** One color per zone that calls for attention; quiet zones stay neutral. */
const ZONE_COLORS: Readonly<Record<BalanceZone, string>> = {
  pain: DIAGRAM_COLORS.danger,
  useless: DIAGRAM_COLORS.warning,
  drifting: DIAGRAM_COLORS.inkSecondary,
  balanced: DIAGRAM_COLORS.accent,
  free: DIAGRAM_COLORS.inkTertiary,
  isolated: DIAGRAM_COLORS.inkTertiary,
};

/** How many components sit in one zone. */
export interface ZoneSummary {
  readonly zone: BalanceZone;
  readonly title: string;
  readonly meaning: string;
  readonly count: number;
}

/** One component of the balance ranking. */
export interface BalanceEntry {
  readonly id: string;
  readonly name: string;
  /** Category label of the bar; the identifier when two entries share a name. */
  readonly label: string;
  readonly zone: BalanceZone;
  readonly distance: number;
  /** How many components depend on it: the length of the bar. */
  readonly dependents: number;
  readonly color: string;
  /** The sentences the tooltip and the accessible label share. */
  readonly summary: string;
}

/** One bucket of the distance distribution. */
export interface DistanceBucket {
  readonly label: string;
  readonly count: number;
  readonly color: string;
  /** One sentence the tooltip shares with assistive technology. */
  readonly summary: string;
}

/** Counts the components of every zone, in the fixed presentation order. */
export function summarizeZones(components: readonly GraphComponent[]): readonly ZoneSummary[] {
  const counts = new Map<BalanceZone, number>();
  for (const component of components) {
    const zone = readMetrics(component).zone;
    counts.set(zone, (counts.get(zone) ?? 0) + 1);
  }
  return ZONE_ORDER.map((zone) => ({
    zone,
    title: ZONE_DETAILS[zone].title,
    meaning: ZONE_DETAILS[zone].meaning,
    count: counts.get(zone) ?? 0,
  }));
}

// The risk of one unbalanced component: how far it sits from its balance,
// weighted by how many components a change in it reaches.
function risk(component: GraphComponent): number {
  return component.mainSequenceDistance * component.afferentCoupling;
}

/**
 * Ranks the components that call for attention: the corner zones and the ones
 * off the diagonal, the riskiest first. Many components share a distance, so
 * the distance alone orders nothing; the ranking therefore weighs it by the
 * number of dependents. The identifier breaks every tie, so the ranking is
 * total and two runs over the same graph agree.
 */
export function rankOffenders(
  components: readonly GraphComponent[],
  limit = RANKING_LIMIT,
): readonly BalanceEntry[] {
  const ranked = components
    .map((component) => ({ component, reading: readMetrics(component) }))
    .filter(({ reading }) => ['pain', 'useless', 'drifting'].includes(reading.zone))
    .sort(
      ({ component: first }, { component: second }) =>
        risk(second) - risk(first) || first.id.localeCompare(second.id),
    )
    .slice(0, limit);
  const nameCounts = new Map<string, number>();
  for (const { component } of ranked) {
    nameCounts.set(component.name, (nameCounts.get(component.name) ?? 0) + 1);
  }
  return ranked.map(({ component, reading }) => ({
    id: component.id,
    name: component.name,
    label: (nameCounts.get(component.name) ?? 0) > 1 ? component.id : component.name,
    zone: reading.zone,
    distance: component.mainSequenceDistance,
    dependents: component.afferentCoupling,
    color: ZONE_COLORS[reading.zone],
    summary:
      `${component.name}: ${reading.headline}. ` +
      (component.afferentCoupling === 1
        ? '1 component depends on it'
        : `${component.afferentCoupling} components depend on it`) +
      `, ${component.mainSequenceDistance.toFixed(2)} from the balance.`,
  }));
}

/**
 * Buckets the distance from the main sequence of every component whose balance
 * carries meaning. Isolated components and entry points are left out: their
 * distance says nothing a reader can act on.
 */
export function bucketDistances(components: readonly GraphComponent[]): readonly DistanceBucket[] {
  const measured = components.filter((component) =>
    ['balanced', 'drifting', 'pain', 'useless'].includes(readMetrics(component).zone),
  );
  const bucketCount = Math.round(1 / BUCKET_WIDTH);
  return Array.from({ length: bucketCount }, (_, index) => {
    const start = index * BUCKET_WIDTH;
    const end = start + BUCKET_WIDTH;
    const last = index === bucketCount - 1;
    const count = measured.filter(
      ({ mainSequenceDistance: distance }) =>
        distance >= start && (last ? distance <= end : distance < end),
    ).length;
    return {
      label: `${start.toFixed(1)}–${end.toFixed(1)}`,
      count,
      // The balanced range keeps the accent color, so the healthy share of the
      // repository reads at a glance.
      color: start < BALANCED_DISTANCE ? DIAGRAM_COLORS.accent : DIAGRAM_COLORS.inkTertiary,
      summary: `${count} component${count === 1 ? '' : 's'} between ${start.toFixed(1)} and ${end.toFixed(1)}`,
    };
  });
}
