import { computed, DestroyRef, inject, Service, signal } from '@angular/core';
import { ConnectError } from '@connectrpc/connect';
import type { Graph } from 'lib-api-gen/gen/v1/graph_pb';
import { GraphEventType } from 'lib-api-gen/gen/v1/graph_pb';
import { GRAPH_CLIENT, QUALITY_CLIENT } from '../client/api';
import { buildGraphLayout } from './graph-layout';

/** State of the connection that keeps the console current. */
export type LiveState = 'connecting' | 'live' | 'disconnected';

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

  /** Whether the first load has not produced a graph yet. */
  readonly loading = signal(true);

  /**
   * Whether a re-analysis the reader asked for is running.
   *
   * It is reported apart from {@link loading} because the two need opposite
   * treatments: the first load has nothing to show and takes the whole page,
   * while a refresh keeps the current graph readable and only marks the control
   * that started it.
   */
  readonly refreshing = signal(false);

  /** Message of the last graph failure, empty when the last load succeeded. */
  readonly error = signal('');

  /**
   * Message of the last plugin catalog failure.
   *
   * The catalog is reported apart from the graph because it feeds one card of
   * the overview. Its failure must not claim that the whole analysis is
   * unavailable while a complete graph is on screen.
   */
  readonly pluginError = signal('');

  /** State of the connection that delivers re-analysis events. */
  readonly liveState = signal<LiveState>('connecting');

  /** When the held graph arrived, undefined before the first response. */
  readonly lastUpdatedAt = signal<Date | undefined>(undefined);

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

  /** Whether the console holds no graph and cannot explain why yet. */
  readonly unavailable = computed(() => this.graphState() === undefined && this.error() !== '');

  constructor() {
    this.destroyRef.onDestroy(() => this.abortController.abort());
    void this.initialize();
  }

  /** Re-analyzes the repository and replaces the held graph. */
  async refresh(): Promise<void> {
    if (this.refreshing()) {
      return;
    }
    this.refreshing.set(true);
    try {
      await Promise.all([this.load(true), this.loadPlugins()]);
    } finally {
      this.refreshing.set(false);
    }
  }

  private async initialize(): Promise<void> {
    const [loaded] = await Promise.all([this.load(false), this.loadPlugins()]);
    if (!loaded || this.abortController.signal.aborted) {
      this.liveState.set('disconnected');
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
      this.pluginError.set('');
    } catch (error: unknown) {
      if (!this.abortController.signal.aborted) {
        this.pluginError.set(
          ConnectError.from(error).rawMessage || 'The plugin catalog is unavailable.',
        );
      }
    }
  }

  private async load(refresh: boolean): Promise<boolean> {
    const sequence = ++this.loadSequence;
    try {
      const response = await this.client.getGraph(
        { refresh, cacheKey: '', cacheProtocol: 0 },
        { signal: this.abortController.signal },
      );
      if (sequence !== this.loadSequence || response.graph === undefined) {
        return response.graph !== undefined;
      }
      this.graphState.set(response.graph);
      this.lastUpdatedAt.set(new Date());
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
        this.liveState.set('live');
        if (
          event.type === GraphEventType.CHANGED &&
          event.revision !== this.graphState()?.revision
        ) {
          await this.load(false);
        }
      }
      this.liveState.set('disconnected');
    } catch (error: unknown) {
      if (!this.abortController.signal.aborted) {
        this.liveState.set('disconnected');
        this.error.set(ConnectError.from(error).rawMessage || 'Live updates are unavailable.');
      }
    }
  }
}
