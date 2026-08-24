import type {
  IncomingTypeRelation,
  TypeDeclaration,
  TypeReference,
} from 'lib-api-gen/gen/v1/graph_pb';

/** Width, in pixels, of one type node. */
export const TYPE_NODE_WIDTH = 264;

/** Height, in pixels, of the header row of one type node. */
export const TYPE_HEADER_HEIGHT = 36;

/** Height, in pixels, of one field or method row. */
export const TYPE_ROW_HEIGHT = 20;

/** Width, in pixels, of one external type node. */
export const EXTERNAL_NODE_WIDTH = 224;

/** Height, in pixels, of one external type node. */
export const EXTERNAL_NODE_HEIGHT = 48;

const NODE_BOTTOM_PADDING = 8;
const COLUMN_GAP = 96;
const NODE_GAP = 48;
const CANVAS_MARGIN = 24;

/** The relation kinds the diagram draws, each with its own link style. */
export type TypeRelationKind = 'implements' | 'embeds' | 'references';

/** One field or method row of a type node, addressable through its port. */
export interface TypeNodeRow {
  /** Port identifier, unique inside the node. */
  readonly port: string;
  /** Whether the row shows a field or a method. */
  readonly member: 'field' | 'method';
  /** Rendered text of the row. */
  readonly text: string;
  /** Whether the row names an exported member. */
  readonly exported: boolean;
}

/** One declared type of the selected component, positioned on the canvas. */
export interface TypeDiagramNode {
  readonly id: string;
  readonly name: string;
  /** Go declaration kind: struct, interface, alias, or basic. */
  readonly kind: string;
  /** Whether the type itself is exported from its package. */
  readonly exported: boolean;
  /** How many types of other components implement, embed, or reference it. */
  readonly incomingCount: number;
  readonly packagePath: string;
  readonly rows: readonly TypeNodeRow[];
  readonly collapsed: boolean;
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
}

/** One type of another component, drawn as a navigation target. */
export interface ExternalTypeNode {
  readonly id: string;
  readonly name: string;
  /** Identifier of the component that declares the type. */
  readonly componentId: string;
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
}

/** One semantic relation between two drawn nodes. */
export interface TypeDiagramEdge {
  readonly id: string;
  readonly kind: TypeRelationKind;
  readonly sourceId: string;
  /** Port of the member that creates the relation, absent for a collapsed source. */
  readonly sourcePort?: string;
  readonly targetId: string;
}

/** The positioned nodes and semantic links of one component's types. */
export interface TypeDiagramModel {
  readonly nodes: readonly TypeDiagramNode[];
  readonly externals: readonly ExternalTypeNode[];
  readonly edges: readonly TypeDiagramEdge[];
  /** Size the canvas needs to contain every node. */
  readonly width: number;
  readonly height: number;
}

/** Short name of one type identifier, the part after the package path. */
export function typeShortName(identifier: string): string {
  const dot = identifier.lastIndexOf('.');
  return dot === -1 ? identifier : identifier.slice(dot + 1);
}

/**
 * Builds the deterministic diagram model of one component's types.
 *
 * The nodes keep the identifier order of the report. A relation whose target
 * lives in another component produces an external node on the right. An
 * incoming relation — a type of another component that implements or uses one
 * of these types — only adds to a counter on its target: a popular interface
 * would otherwise drown the canvas in arrows. Selecting one type draws its
 * incoming sources as external nodes on the left.
 */
export function buildTypeDiagramModel(
  types: readonly TypeDeclaration[],
  incoming: readonly IncomingTypeRelation[],
  collapsedIds: ReadonlySet<string>,
  selectedTypeId: string,
): TypeDiagramModel {
  const sorted = [...types].sort((first, second) => first.id.localeCompare(second.id));
  const internalIds = new Set(sorted.map((declaration) => declaration.id));
  const relevantIncoming = incoming.filter(
    (relation) => internalIds.has(relation.targetId) && !internalIds.has(relation.sourceId),
  );
  const selectedIncoming = relevantIncoming.filter(
    (relation) => relation.targetId === selectedTypeId,
  );
  const incomingCounts = new Map<string, number>();
  for (const relation of relevantIncoming) {
    incomingCounts.set(relation.targetId, (incomingCounts.get(relation.targetId) ?? 0) + 1);
  }

  const externalTargets = collectExternalTargets(sorted, internalIds);
  // The incoming sources open a column at the left, so the internal columns
  // shift right by its width.
  const offset = selectedIncoming.length === 0 ? 0 : EXTERNAL_NODE_WIDTH + COLUMN_GAP;
  const nodes = layoutNodes(sorted, collapsedIds, offset, incomingCounts);
  const outgoingExternals = layoutExternals(externalTargets, nodes);
  const incomingExternals = layoutIncomingSources(selectedIncoming, outgoingExternals);
  const externals = [...incomingExternals, ...outgoingExternals];
  const edges = [...buildIncomingEdges(selectedIncoming), ...buildEdges(sorted, collapsedIds)];

  const width =
    Math.max(
      ...nodes.map((node) => node.x + node.width),
      ...externals.map((node) => node.x + node.width),
      0,
    ) + CANVAS_MARGIN;
  const height =
    Math.max(
      ...nodes.map((node) => node.y + node.height),
      ...externals.map((node) => node.y + node.height),
      0,
    ) + CANVAS_MARGIN;

  return { nodes, externals, edges, width, height };
}

// The UML visibility markers: Go exports through the capital initial, which a
// reader should not have to inspect, so every row and header states it.
function visibilityMarker(exported: boolean): string {
  return exported ? '+' : '−';
}

function nodeRows(declaration: TypeDeclaration): TypeNodeRow[] {
  const fields = declaration.fields.map(
    (field): TypeNodeRow => ({
      port: `field:${field.name}`,
      member: 'field',
      text: `${visibilityMarker(field.exported)} ${field.name}: ${field.type}`,
      exported: field.exported,
    }),
  );
  const methods = declaration.methods.map(
    (method): TypeNodeRow => ({
      port: `method:${method.name}`,
      member: 'method',
      text: `${visibilityMarker(method.exported)} ${method.name}${method.signature}`,
      exported: method.exported,
    }),
  );
  return [...fields, ...methods];
}

function nodeHeight(rowCount: number, collapsed: boolean): number {
  if (collapsed || rowCount === 0) {
    return TYPE_HEADER_HEIGHT;
  }
  return TYPE_HEADER_HEIGHT + rowCount * TYPE_ROW_HEIGHT + NODE_BOTTOM_PADDING;
}

// layoutNodes fills deterministic columns: each node lands in the least
// filled column, and a tie selects the leftmost one.
function layoutNodes(
  sorted: readonly TypeDeclaration[],
  collapsedIds: ReadonlySet<string>,
  offset: number,
  incomingCounts: ReadonlyMap<string, number>,
): TypeDiagramNode[] {
  const columnCount = Math.min(4, Math.max(1, Math.ceil(Math.sqrt(sorted.length))));
  const columnHeights = Array.from({ length: columnCount }, () => CANVAS_MARGIN);
  return sorted.map((declaration) => {
    const collapsed = collapsedIds.has(declaration.id);
    const rows = nodeRows(declaration);
    const height = nodeHeight(rows.length, collapsed);
    const column = columnHeights.indexOf(Math.min(...columnHeights));
    const x = CANVAS_MARGIN + offset + column * (TYPE_NODE_WIDTH + COLUMN_GAP);
    const y = columnHeights[column];
    columnHeights[column] = y + height + NODE_GAP;
    return {
      id: declaration.id,
      name: declaration.name,
      kind: declaration.kind,
      exported: declaration.exported,
      incomingCount: incomingCounts.get(declaration.id) ?? 0,
      packagePath: declaration.package,
      rows: collapsed ? [] : rows,
      collapsed,
      x,
      y,
      width: TYPE_NODE_WIDTH,
      height,
    };
  });
}

// layoutExternals stacks the navigation targets in one column at the right
// of the internal nodes.
function layoutExternals(
  targets: readonly TypeReference[],
  nodes: readonly TypeDiagramNode[],
): ExternalTypeNode[] {
  const x = Math.max(...nodes.map((node) => node.x + node.width), CANVAS_MARGIN) + COLUMN_GAP;
  return targets.map((target, index) => ({
    id: target.id,
    name: typeShortName(target.id),
    componentId: target.component,
    x,
    y: CANVAS_MARGIN + index * (EXTERNAL_NODE_HEIGHT + NODE_GAP / 2),
    width: EXTERNAL_NODE_WIDTH,
    height: EXTERNAL_NODE_HEIGHT,
  }));
}

// layoutIncomingSources stacks the incoming implementers and users in one
// column at the left. A source that is already an outgoing target keeps its
// node at the right, so no identifier appears twice on the canvas.
function layoutIncomingSources(
  relevantIncoming: readonly IncomingTypeRelation[],
  outgoingExternals: readonly ExternalTypeNode[],
): ExternalTypeNode[] {
  const placed = new Set(outgoingExternals.map((node) => node.id));
  const sources = new Map<string, string>();
  for (const relation of relevantIncoming) {
    if (!placed.has(relation.sourceId)) {
      sources.set(relation.sourceId, relation.sourceComponent);
    }
  }
  return [...sources.entries()]
    .sort(([first], [second]) => first.localeCompare(second))
    .map(([id, componentId], index) => ({
      id,
      name: typeShortName(id),
      componentId,
      x: CANVAS_MARGIN,
      y: CANVAS_MARGIN + index * (EXTERNAL_NODE_HEIGHT + NODE_GAP / 2),
      width: EXTERNAL_NODE_WIDTH,
      height: EXTERNAL_NODE_HEIGHT,
    }));
}

// buildIncomingEdges points every incoming relation at the internal target,
// so the arrow keeps the Go direction: the implementer aims at the interface.
function buildIncomingEdges(
  relevantIncoming: readonly IncomingTypeRelation[],
): TypeDiagramEdge[] {
  return relevantIncoming.map((relation) => ({
    id: `incoming:${relation.kind}:${relation.sourceId}->${relation.targetId}`,
    kind: relation.kind as TypeRelationKind,
    sourceId: relation.sourceId,
    targetId: relation.targetId,
  }));
}

function collectExternalTargets(
  sorted: readonly TypeDeclaration[],
  internalIds: ReadonlySet<string>,
): TypeReference[] {
  const targets = new Map<string, TypeReference>();
  for (const declaration of sorted) {
    for (const reference of [
      ...declaration.implements,
      ...declaration.embeds,
      ...declaration.references,
    ]) {
      if (!internalIds.has(reference.id) && !targets.has(reference.id)) {
        targets.set(reference.id, reference);
      }
    }
  }
  return [...targets.values()].sort((first, second) => first.id.localeCompare(second.id));
}

function buildEdges(
  sorted: readonly TypeDeclaration[],
  collapsedIds: ReadonlySet<string>,
): TypeDiagramEdge[] {
  const edges: TypeDiagramEdge[] = [];
  for (const declaration of sorted) {
    const collapsed = collapsedIds.has(declaration.id);
    for (const reference of declaration.implements) {
      edges.push(edge('implements', declaration, reference, undefined));
    }
    for (const reference of declaration.embeds) {
      edges.push(
        edge(
          'embeds',
          declaration,
          reference,
          collapsed ? undefined : memberPort(declaration, reference, true),
        ),
      );
    }
    for (const reference of declaration.references) {
      edges.push(
        edge(
          'references',
          declaration,
          reference,
          collapsed ? undefined : memberPort(declaration, reference, false),
        ),
      );
    }
  }
  return edges;
}

function edge(
  kind: TypeRelationKind,
  declaration: TypeDeclaration,
  reference: TypeReference,
  sourcePort: string | undefined,
): TypeDiagramEdge {
  return {
    id: `${kind}:${declaration.id}->${reference.id}`,
    kind,
    sourceId: declaration.id,
    sourcePort,
    targetId: reference.id,
  };
}

// memberPort anchors one relation at the field that produces it. The report
// does not name the field, so the field type text is matched against the
// short name of the target; the node header stays the fallback anchor.
function memberPort(
  declaration: TypeDeclaration,
  reference: TypeReference,
  embedded: boolean,
): string | undefined {
  const shortName = typeShortName(reference.id);
  const namePattern = new RegExp(`\\b${escapePattern(shortName)}\\b`);
  const field = declaration.fields.find(
    (candidate) => candidate.embedded === embedded && namePattern.test(candidate.type),
  );
  return field === undefined ? undefined : `field:${field.name}`;
}

function escapePattern(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
