import { create, type MessageInitShape } from '@bufbuild/protobuf';
import {
  ComponentSchema,
  FindingSchema,
  GraphSchema,
  GraphSummarySchema,
  IncomingTypeRelationSchema,
  RelationshipSchema,
  TypeDeclarationSchema,
  type Component,
  type Finding,
  type Graph,
  type IncomingTypeRelation,
  type Relationship,
  type TypeDeclaration,
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

/** Builds one dependency between two components. */
export function fakeRelationship(source: string, target: string, testOnly = false): Relationship {
  return create(RelationshipSchema, { source, target, testOnly });
}

/** Builds one rule result with the fields the console reads. */
export function fakeFinding(rule: string, severity: string, subject: string): Finding {
  return create(FindingSchema, { rule, severity, subject, message: `${rule} is not satisfied` });
}

/** The fields of a type declaration a test may set, without the Protobuf bookkeeping. */
type TypeDeclarationParts = MessageInitShape<typeof TypeDeclarationSchema>;

/** Builds one declared Go type with the fields the console reads. */
export function fakeTypeDeclaration(
  id: string,
  kind: string,
  parts: TypeDeclarationParts = {},
): TypeDeclaration {
  const dot = id.lastIndexOf('.');
  return create(TypeDeclarationSchema, {
    id,
    name: dot === -1 ? id : id.slice(dot + 1),
    package: dot === -1 ? '' : id.slice(0, dot),
    component: dot === -1 ? '' : id.slice(0, dot),
    kind,
    exported: true,
    ...parts,
  });
}

// incomingRelationsOf mirrors the server: it collects every relation of
// another component whose target the requested component declares.
function incomingRelationsOf(
  declarations: readonly TypeDeclaration[],
  componentId: string,
): IncomingTypeRelation[] {
  const incoming: IncomingTypeRelation[] = [];
  for (const declaration of declarations) {
    if (declaration.component === componentId) {
      continue;
    }
    const groups: readonly (readonly [string, readonly { id: string; component: string }[]])[] = [
      ['implements', declaration.implements],
      ['embeds', declaration.embeds],
      ['references', declaration.references],
    ];
    for (const [kind, references] of groups) {
      for (const reference of references) {
        if (reference.component === componentId) {
          incoming.push(
            create(IncomingTypeRelationSchema, {
              kind,
              sourceId: declaration.id,
              sourceComponent: declaration.component,
              targetId: reference.id,
            }),
          );
        }
      }
    }
  }
  return incoming;
}

/** Builds one incoming type relation with the fields the console reads. */
export function fakeIncomingTypeRelation(
  kind: string,
  sourceId: string,
  targetId: string,
): IncomingTypeRelation {
  const dot = sourceId.lastIndexOf('.');
  return create(IncomingTypeRelationSchema, {
    kind,
    sourceId,
    sourceComponent: dot === -1 ? '' : sourceId.slice(0, dot),
    targetId,
  });
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
export function provideFakeClients(
  graph: Graph | undefined,
  plugins: string[] = [],
  componentTypes: TypeDeclaration[] = [],
): unknown[] {
  return [
    {
      provide: GRAPH_CLIENT,
      useValue: {
        getGraph: () => Promise.resolve({ graph }),
        // The watch stream never yields, so a test observes the first load only.
        watchGraph: () => ({
          [Symbol.asyncIterator]: () => ({ next: () => new Promise(() => undefined) }),
        }),
        getComponentTypes: (request: { componentId: string }) =>
          Promise.resolve({
            revision: graph?.revision ?? '',
            componentId: request.componentId,
            types: componentTypes.filter(
              (declaration) => declaration.component === request.componentId,
            ),
            incoming: incomingRelationsOf(componentTypes, request.componentId),
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
