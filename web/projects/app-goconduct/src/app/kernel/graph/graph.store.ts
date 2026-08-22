import { computed, DestroyRef, inject, Service, signal } from '@angular/core';
import { ConnectError } from '@connectrpc/connect';
import type { Graph } from 'lib-api-gen/gen/v1/graph_pb';
import { GraphEventType } from 'lib-api-gen/gen/v1/graph_pb';
import { GRAPH_CLIENT, QUALITY_CLIENT } from '../client/api';
import { buildGraphLayout } from './graph-layout';

/**
 * The analyzed repository, held once for the whole console.
 *
 * The store is application-scoped rather than page-scoped for two reasons. The
 * graph is one document that every page reads, so a page-scoped store would
 * fetch it again on each navigation. The watch stream is also long-lived: it
 * keeps the console current while the server re-analyzes the repository, and a
 * page-scoped store would drop it at the first navigation.
 */
@Service()
export class GraphStore {
  private readonly client = inject(GRAPH_CLIENT);
  private readonly qualityClient = inject(QUALITY_CLIENT);
  private readonly destroyRef = inject(DestroyRef);
  private readonly abortController = new AbortController();
  // Identifies the most recent load, so a slow earlier response never overwrites
  // the graph a later one already published.
  private loadSequence = 0;

  private readonly graphState = signal<Graph | undefined>(undefined);

  /** The analyzed graph, undefined until the first response arrives. */
  readonly graph = this.graphState.asReadonly();

  /** Whether a load is in flight. */
  readonly loading = signal(true);

  /** Message of the last failure, empty when the last operation succeeded. */
  readonly error = signal('');

  /** Whether the watch stream delivers events. */
  readonly live = signal(false);

  /** Names of the quality plugins the server reports. */
  readonly qualityPlugins = signal<readonly string[]>([]);

  /** Every analyzed component, in the order the server reports. */
  readonly components = computed(() => this.graphState()?.components ?? []);

  /** Every analyzed dependency. */
  readonly relationships = computed(() => this.graphState()?.relationships ?? []);

  /** Every result of the deterministic architecture rules. */
  readonly findings = computed(() => this.graphState()?.findings ?? []);

  /** Counters of the analysis, undefined until the first response arrives. */
  readonly summary = computed(() => this.graphState()?.summary);

  /** Distinct component roles, sorted, for the components filter. */
  readonly roles = computed(() =>
    [...new Set(this.components().map((component) => component.role))]
      .filter((role) => role !== '')
      .sort((first, second) => first.localeCompare(second)),
  );

  /** Positioned map of the most connected components. */
  readonly layout = computed(() => buildGraphLayout(this.components(), this.relationships()));

  constructor() {
    this.destroyRef.onDestroy(() => this.abortController.abort());
    void this.initialize();
  }

  /** Re-analyzes the repository and replaces the held graph. */
  async refresh(): Promise<void> {
    await this.load(true);
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
