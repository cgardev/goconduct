import type { dia } from '@joint/core';
import { DIAGRAM_COLORS, typeKindColor } from '../../../kernel/diagram/diagram-colors';
import {
  TYPE_HEADER_HEIGHT,
  TYPE_ROW_HEIGHT,
  type ExternalTypeNode,
  type TypeDiagramEdge,
  type TypeDiagramModel,
  type TypeDiagramNode,
} from './type-diagram.model';

const MINIMUM_SCALE = 0.3;
const MAXIMUM_SCALE = 2.5;

/** What the diagram reports back to the hosting component. */
export interface TypeDiagramCallbacks {
  /** The reader clicked one type of the component. */
  onSelect(typeId: string): void;
  /** The reader clicked the collapse control of one type. */
  onToggle(typeId: string): void;
  /** The reader clicked one type of another component. */
  onNavigate(componentId: string): void;
}

/** The rendered diagram, ready to receive a model and a selection. */
export interface TypeDiagram {
  /** Replaces the nodes and links and marks the selected type. */
  render(model: TypeDiagramModel, selectedId: string): void;
  /** Multiplies the zoom around the canvas center. */
  zoom(factor: number): void;
  /** Restores the scale and position that show the whole diagram. */
  resetView(): void;
  /** Releases the diagram engine; call it when the component is destroyed. */
  dispose(): void;
}

/**
 * Builds the type diagram with JointJS (`@joint/core`), as `LIBS.md` assigns
 * to UML-style type diagrams: one custom node per type with a header row,
 * one row per field or method with its own port, and one link style per
 * relation kind. The engine loads on demand, so the first page paint does
 * not carry it.
 */
export async function createTypeDiagram(
  element: HTMLElement,
  callbacks: TypeDiagramCallbacks,
): Promise<TypeDiagram> {
  const joint = await import('@joint/core');
  const typeNode = defineTypeNode(joint.dia);
  const namespace = { ...joint.shapes, goconduct: { TypeNode: typeNode } };

  const graph = new joint.dia.Graph({}, { cellNamespace: namespace });
  const paper = new joint.dia.Paper({
    el: element,
    model: graph,
    width: element.clientWidth,
    height: element.clientHeight,
    cellViewNamespace: namespace,
    // The layout is deterministic, so the nodes stay where the model puts them.
    interactive: false,
    background: { color: 'transparent' },
  });

  paper.on('element:pointerclick', (view: dia.ElementView, event: dia.Event) => {
    const target = event.target as Element | null;
    const typeId: unknown = view.model.get('typeId');
    if (typeof typeId !== 'string') {
      return;
    }
    if (target?.closest('[data-role="toggle"]') !== null) {
      callbacks.onToggle(typeId);
      return;
    }
    const componentId: unknown = view.model.get('externalComponentId');
    if (typeof componentId === 'string' && componentId !== '') {
      callbacks.onNavigate(componentId);
      return;
    }
    callbacks.onSelect(typeId);
  });

  const zoomAt = (factor: number, x: number, y: number): void => {
    const current = paper.scale().sx;
    const next = Math.min(MAXIMUM_SCALE, Math.max(MINIMUM_SCALE, current * factor));
    const translate = paper.translate();
    // The point under the pointer stays fixed while the scale changes.
    paper.translate(x - ((x - translate.tx) / current) * next, y - ((y - translate.ty) / current) * next);
    paper.scale(next);
  };
  const wheel = (event: dia.Event, x: number, y: number, delta: number): void => {
    event.preventDefault();
    const translate = paper.translate();
    const scale = paper.scale().sx;
    zoomAt(delta > 0 ? 1.1 : 1 / 1.1, x * scale + translate.tx, y * scale + translate.ty);
  };
  paper.on('blank:mousewheel', wheel);
  paper.on('cell:mousewheel', (_view: dia.CellView, event: dia.Event, x: number, y: number, delta: number) =>
    wheel(event, x, y, delta),
  );

  // Dragging the blank surface pans the canvas; the drag never leaves it.
  let panning: { pointer: { x: number; y: number }; translate: { tx: number; ty: number } } | undefined;
  paper.on('blank:pointerdown', (event: dia.Event) => {
    panning = {
      pointer: { x: event.clientX ?? 0, y: event.clientY ?? 0 },
      translate: paper.translate(),
    };
  });
  const move = (event: PointerEvent): void => {
    if (panning === undefined) {
      return;
    }
    paper.translate(
      panning.translate.tx + event.clientX - panning.pointer.x,
      panning.translate.ty + event.clientY - panning.pointer.y,
    );
  };
  const stop = (): void => {
    panning = undefined;
  };
  element.addEventListener('pointermove', move);
  element.addEventListener('pointerup', stop);
  element.addEventListener('pointerleave', stop);

  let lastModel: TypeDiagramModel = { nodes: [], externals: [], edges: [], width: 0, height: 0 };
  let lastSelectedId = '';

  const draw = (): void => {
    paper.setDimensions(element.clientWidth, element.clientHeight);
    const nodeIds = new Set([
      ...lastModel.nodes.map((node) => node.id),
      ...lastModel.externals.map((node) => node.id),
    ]);
    graph.resetCells([
      ...lastModel.edges
        .filter((edge) => nodeIds.has(edge.sourceId) && nodeIds.has(edge.targetId))
        .map((edge) => buildLink(joint.shapes, edge)),
      ...lastModel.nodes.map((node) => buildTypeNode(typeNode, node, node.id === lastSelectedId)),
      ...lastModel.externals.map((node) => buildExternalNode(typeNode, node)),
    ]);
  };

  const observer = new ResizeObserver(() => draw());
  observer.observe(element);

  return {
    render(model, selectedId): void {
      lastModel = model;
      lastSelectedId = selectedId;
      draw();
    },
    zoom(factor): void {
      zoomAt(factor, element.clientWidth / 2, element.clientHeight / 2);
    },
    resetView(): void {
      paper.scale(1);
      paper.translate(0, 0);
    },
    dispose(): void {
      observer.disconnect();
      element.removeEventListener('pointermove', move);
      element.removeEventListener('pointerup', stop);
      element.removeEventListener('pointerleave', stop);
      paper.remove();
    },
  };
}

type TypeNodeConstructor = ReturnType<typeof defineTypeNode>;

// The markup of a type node is built per instance, because the row count
// varies; the shared definition only names the type for the cell namespace.
function defineTypeNode(diaNamespace: typeof dia) {
  return diaNamespace.Element.define('goconduct.TypeNode', {}, {});
}

function buildTypeNode(
  typeNode: TypeNodeConstructor,
  node: TypeDiagramNode,
  selected: boolean,
): dia.Element {
  const headerColor = typeKindColor(node.kind);
  const markup: dia.MarkupJSON = [
    { tagName: 'rect', selector: 'body' },
    { tagName: 'rect', selector: 'header' },
    { tagName: 'text', selector: 'name' },
    { tagName: 'text', selector: 'kind' },
    { tagName: 'text', selector: 'toggle' },
    ...node.rows.map((_, index) => ({ tagName: 'text', selector: `row${index}` })),
  ];
  const attrs: Record<string, Record<string, unknown>> = {
    body: {
      width: 'calc(w)',
      height: 'calc(h)',
      rx: 6,
      fill: DIAGRAM_COLORS.surface,
      stroke: selected ? DIAGRAM_COLORS.focus : DIAGRAM_COLORS.lineStrong,
      strokeWidth: selected ? 2 : 1,
      cursor: 'pointer',
    },
    header: {
      width: 'calc(w)',
      height: TYPE_HEADER_HEIGHT,
      rx: 6,
      fill: headerColor,
      cursor: 'pointer',
    },
    name: {
      x: 10,
      y: TYPE_HEADER_HEIGHT / 2 + 1,
      textVerticalAnchor: 'middle',
      fill: DIAGRAM_COLORS.surface,
      fontSize: 12,
      fontWeight: '700',
      text: `${node.exported ? '+' : '−'} ${node.name}`,
      pointerEvents: 'none',
    },
    kind: {
      x: 'calc(w-30)',
      y: TYPE_HEADER_HEIGHT / 2 + 1,
      textAnchor: 'end',
      textVerticalAnchor: 'middle',
      fill: DIAGRAM_COLORS.surface,
      fontSize: 10,
      fontStyle: 'italic',
      // The counter names the uses that arrive from other components; the
      // arrows themselves only appear for the selected type.
      text:
        node.incomingCount > 0 ? `◂${node.incomingCount} «${node.kind}»` : `«${node.kind}»`,
      pointerEvents: 'none',
    },
    toggle: {
      'x': 'calc(w-14)',
      'y': TYPE_HEADER_HEIGHT / 2 + 1,
      'textAnchor': 'middle',
      'textVerticalAnchor': 'middle',
      'fill': DIAGRAM_COLORS.surface,
      'fontSize': 13,
      'fontWeight': '700',
      // A chevron, not a plus or minus: those glyphs now mark visibility.
      'text': node.collapsed ? '▸' : '▾',
      'cursor': 'pointer',
      'data-role': 'toggle',
    },
  };
  node.rows.forEach((row, index) => {
    attrs[`row${index}`] = {
      x: 10,
      y: TYPE_HEADER_HEIGHT + (index + 0.5) * TYPE_ROW_HEIGHT + 3,
      textVerticalAnchor: 'middle',
      fill: row.exported ? DIAGRAM_COLORS.ink : DIAGRAM_COLORS.inkTertiary,
      fontSize: 11,
      fontFamily: 'ui-monospace, SFMono-Regular, Consolas, monospace',
      fontStyle: row.member === 'method' ? 'italic' : 'normal',
      text: clipRow(row.text),
      pointerEvents: 'none',
    };
  });
  return new typeNode({
    id: node.id,
    typeId: node.id,
    position: { x: node.x, y: node.y },
    size: { width: node.width, height: node.height },
    z: 2,
    markup,
    attrs,
    ports: {
      groups: {
        row: {
          position: { name: 'absolute' },
          attrs: {
            portBody: {
              r: 3,
              magnet: 'passive',
              fill: DIAGRAM_COLORS.lineStrong,
              stroke: DIAGRAM_COLORS.surface,
              strokeWidth: 1,
            },
          },
          markup: [{ tagName: 'circle', selector: 'portBody' }],
        },
      },
      items: node.rows.map((row, index) => ({
        id: row.port,
        group: 'row',
        args: { x: 0, y: TYPE_HEADER_HEIGHT + (index + 0.5) * TYPE_ROW_HEIGHT },
      })),
    },
  });
}

function buildExternalNode(typeNode: TypeNodeConstructor, node: ExternalTypeNode): dia.Element {
  return new typeNode({
    id: node.id,
    typeId: node.id,
    externalComponentId: node.componentId,
    position: { x: node.x, y: node.y },
    size: { width: node.width, height: node.height },
    z: 2,
    markup: [
      { tagName: 'rect', selector: 'body' },
      { tagName: 'text', selector: 'name' },
      { tagName: 'text', selector: 'component' },
    ],
    attrs: {
      body: {
        width: 'calc(w)',
        height: 'calc(h)',
        rx: 6,
        fill: DIAGRAM_COLORS.surface,
        stroke: typeKindColor('external'),
        strokeDasharray: '5 3',
        cursor: 'pointer',
      },
      name: {
        x: 10,
        y: 18,
        fill: DIAGRAM_COLORS.ink,
        fontSize: 12,
        fontWeight: '700',
        text: node.name,
        pointerEvents: 'none',
      },
      component: {
        x: 10,
        y: 36,
        fill: DIAGRAM_COLORS.inkSecondary,
        fontSize: 10,
        fontFamily: 'ui-monospace, SFMono-Regular, Consolas, monospace',
        text: node.componentId,
        pointerEvents: 'none',
      },
    },
  });
}

/** Stroke, dash pattern, and markers of one relation kind. */
const EDGE_STYLES: Record<TypeDiagramEdge['kind'], Record<string, unknown>> = {
  implements: {
    stroke: DIAGRAM_COLORS.accent,
    strokeWidth: 1.5,
    strokeDasharray: '6 3',
    targetMarker: {
      type: 'path',
      d: 'M 12 -6 0 0 12 6 z',
      fill: DIAGRAM_COLORS.surface,
      stroke: DIAGRAM_COLORS.accent,
    },
  },
  embeds: {
    stroke: DIAGRAM_COLORS.inkSecondary,
    strokeWidth: 1.5,
    strokeDasharray: 'none',
    sourceMarker: {
      type: 'path',
      d: 'M 0 0 8 -5 16 0 8 5 z',
      fill: DIAGRAM_COLORS.inkSecondary,
      stroke: DIAGRAM_COLORS.inkSecondary,
    },
    targetMarker: { type: 'path', d: 'M 8 -4 0 0 8 4', fill: 'none' },
  },
  references: {
    stroke: DIAGRAM_COLORS.inkTertiary,
    strokeWidth: 1.25,
    strokeDasharray: '2 3',
    targetMarker: { type: 'path', d: 'M 8 -4 0 0 8 4', fill: 'none' },
  },
};

function buildLink(
  shapes: typeof import('@joint/core').shapes,
  edge: TypeDiagramEdge,
): dia.Link {
  return new shapes.standard.Link({
    source:
      edge.sourcePort === undefined
        ? { id: edge.sourceId }
        : { id: edge.sourceId, port: edge.sourcePort },
    target: { id: edge.targetId },
    z: 1,
    connector: { name: 'rounded' },
    attrs: { line: EDGE_STYLES[edge.kind] },
  });
}

// clipRow keeps a row inside the node width; the full text stays readable
// in the detail panel and in the accessible list.
function clipRow(text: string): string {
  const limit = 34;
  return text.length <= limit ? text : `${text.slice(0, limit - 1)}…`;
}
