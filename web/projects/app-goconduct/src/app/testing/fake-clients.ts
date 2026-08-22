import { create } from '@bufbuild/protobuf';
import {
  ComponentSchema,
  FindingSchema,
  GraphSchema,
  GraphSummarySchema,
  type Component,
  type Finding,
  type Graph,
} from 'lib-api-gen/gen/v1/graph_pb';
import { GRAPH_CLIENT, QUALITY_CLIENT } from '../kernel/client/api';

/** Builds one analyzed component with the fields the console reads. */
export function fakeComponent(id: string, role: string, afferentCoupling = 0): Component {
  return create(ComponentSchema, {
    id,
    name: id.split('/').at(-1) ?? id,
    role,
    afferentCoupling,
  });
}

/** Builds one rule result with the fields the console reads. */
export function fakeFinding(rule: string, severity: string, subject: string): Finding {
  return create(FindingSchema, { rule, severity, subject, message: `${rule} is not satisfied` });
}

/** The fields of a graph a test may set, without the Protobuf bookkeeping. */
type GraphParts = Partial<Omit<Graph, '$typeName' | '$unknown'>>;

/** Builds an analyzed graph from the parts a test cares about. */
export function fakeGraph(parts: GraphParts = {}): Graph {
  return create(GraphSchema, {
    revision: 'abcdef123456',
    modulePath: 'github.com/cgardev/goconduct',
    summary: create(GraphSummarySchema, {}),
    ...parts,
  });
}

/**
 * Replaces the generated Connect clients with fixed responses.
 *
 * The store is left alone on purpose: a test that swapped the store would stop
 * covering the code that turns a response into the signals every page reads.
 * The seam is therefore the transport, which is the one boundary a test cannot
 * cross anyway.
 */
export function provideFakeClients(graph: Graph | undefined, plugins: string[] = []): unknown[] {
  return [
    {
      provide: GRAPH_CLIENT,
      useValue: {
        getGraph: () => Promise.resolve({ graph }),
        // The watch stream never yields, so a test observes the first load only.
        watchGraph: () => ({
          [Symbol.asyncIterator]: () => ({ next: () => new Promise(() => undefined) }),
        }),
      },
    },
    {
      provide: QUALITY_CLIENT,
      useValue: {
        listPlugins: () => Promise.resolve({ plugins: plugins.map((name) => ({ name })) }),
      },
    },
  ];
}
