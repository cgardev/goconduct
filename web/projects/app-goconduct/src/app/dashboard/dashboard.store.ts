import { computed, DestroyRef, inject, Injectable, signal } from '@angular/core';
import { ConnectError } from '@connectrpc/connect';
import type {
  Component as GraphComponent,
  Finding,
  Graph,
  Relationship,
} from 'lib-api-gen/gen/v1/graph_pb';
import { GraphEventType } from 'lib-api-gen/gen/v1/graph_pb';
import { GRAPH_CLIENT } from '../api/graph-client';
import { QUALITY_CLIENT } from '../api/quality-client';

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

/** Filters components by role and a case-insensitive text query. */
export function filterComponents(
  components: readonly GraphComponent[],
  role: string,
  query: string,
): GraphComponent[] {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  return components
    .filter((component) => role === 'all' || component.role === role)
    .filter((component) => {
      if (normalizedQuery === '') {
        return true;
      }
      return [component.id, component.name, component.category, component.application]
        .join(' ')
        .toLocaleLowerCase()
        .includes(normalizedQuery);
    })
    .sort(
      (first, second) =>
        second.afferentCoupling - first.afferentCoupling || first.id.localeCompare(second.id),
    );
}

/** Builds a stable circular layout for the most connected components. */
export function buildGraphLayout(
  components: readonly GraphComponent[],
  relationships: readonly Relationship[],
  limit = 18,
): GraphLayout {
  const selected = [...components]
    .sort(
      (first, second) =>
        second.afferentCoupling + second.efferentCoupling -
          (first.afferentCoupling + first.efferentCoupling) ||
        first.id.localeCompare(second.id),
    )
    .slice(0, limit);
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

/** Holds the dashboard state and coordinates the generated Connect client. */
@Injectable()
export class DashboardStore {
  private readonly client = inject(GRAPH_CLIENT);
  private readonly qualityClient = inject(QUALITY_CLIENT);
  private readonly destroyRef = inject(DestroyRef);
  private readonly abortController = new AbortController();
  private loadSequence = 0;

  private readonly graphState = signal<Graph | undefined>(undefined);
  readonly graph = this.graphState.asReadonly();
  readonly loading = signal(true);
  readonly error = signal('');
  readonly live = signal(false);
  readonly qualityPlugins = signal<readonly string[]>([]);
  readonly query = signal('');
  readonly role = signal('all');
  readonly selectedComponentId = signal('');

  readonly components = computed(() =>
    filterComponents(this.graphState()?.components ?? [], this.role(), this.query()),
  );
  readonly findings = computed(() => this.graphState()?.findings ?? []);
  readonly summary = computed(() => this.graphState()?.summary);
  readonly roles = computed(() =>
    [...new Set((this.graphState()?.components ?? []).map((component) => component.role))]
      .filter((role) => role !== '')
      .sort((first, second) => first.localeCompare(second)),
  );
  readonly layout = computed(() =>
    buildGraphLayout(
      this.graphState()?.components ?? [],
      this.graphState()?.relationships ?? [],
    ),
  );
  readonly selectedComponent = computed(() => {
    const identifier = this.selectedComponentId();
    return this.graphState()?.components.find((component) => component.id === identifier);
  });
  readonly selectedRelationships = computed(() => {
    const identifier = this.selectedComponentId();
    if (identifier === '') {
      return [];
    }
    return (this.graphState()?.relationships ?? []).filter(
      (relationship) =>
        relationship.source === identifier || relationship.target === identifier,
    );
  });

  constructor() {
    this.destroyRef.onDestroy(() => this.abortController.abort());
    void this.initialize();
  }

  setQuery(value: string): void {
    this.query.set(value);
  }

  setRole(value: string): void {
    this.role.set(value);
  }

  selectComponent(identifier: string): void {
    this.selectedComponentId.set(identifier);
  }

  closeComponent(): void {
    this.selectedComponentId.set('');
  }

  async refresh(): Promise<void> {
    await this.load(true);
  }

  findingTone(finding: Finding): 'error' | 'warning' {
    return finding.severity === 'error' ? 'error' : 'warning';
  }

  private async initialize(): Promise<void> {
    const [loaded] = await Promise.all([this.load(false), this.loadPlugins()]);
    if (!loaded || this.abortController.signal.aborted) {
      return;
    }
    await this.watch();
  }

  private async loadPlugins(): Promise<void> {
    try {
      const response = await this.qualityClient.listPlugins(
        {},
        { signal: this.abortController.signal },
      );
      this.qualityPlugins.set(response.plugins.map((candidate) => candidate.name));
    } catch (error: unknown) {
      if (!this.abortController.signal.aborted) {
        this.error.set(ConnectError.from(error).rawMessage || 'The plugin catalog is unavailable.');
      }
    }
  }

  private async load(refresh: boolean): Promise<boolean> {
    const sequence = ++this.loadSequence;
    this.loading.set(true);
    try {
      const response = await this.client.getGraph(
        { refresh, cacheKey: '', cacheProtocol: 0 },
        { signal: this.abortController.signal },
      );
      if (sequence !== this.loadSequence || response.graph === undefined) {
        return response.graph !== undefined;
      }
      this.graphState.set(response.graph);
      this.error.set('');
      return true;
    } catch (error: unknown) {
      if (this.abortController.signal.aborted) {
        return false;
      }
      this.error.set(ConnectError.from(error).rawMessage || 'The graph is unavailable.');
      return false;
    } finally {
      if (sequence === this.loadSequence) {
        this.loading.set(false);
      }
    }
  }

  private async watch(): Promise<void> {
    try {
      const events = this.client.watchGraph(
        { fromRevision: this.graphState()?.revision ?? '' },
        { signal: this.abortController.signal },
      );
      for await (const event of events) {
        this.live.set(true);
        if (
          event.type === GraphEventType.CHANGED &&
          event.revision !== this.graphState()?.revision
        ) {
          await this.load(false);
        }
      }
    } catch (error: unknown) {
      if (!this.abortController.signal.aborted) {
        this.live.set(false);
        this.error.set(ConnectError.from(error).rawMessage || 'Live updates are unavailable.');
      }
    }
  }
}
