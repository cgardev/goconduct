import type { dia } from '@joint/core';
import { DIAGRAM_COLORS, roleColor } from '../../../kernel/diagram/diagram-colors';
import type { GraphLayout } from '../../../kernel/graph/graph-layout';

/** Side, in pixels, of one component node. */
const NODE_SIDE = 44;

/** The rendered diagram, ready to receive a layout and a selection. */
export interface DependencyMapDiagram {
  /** Replaces the nodes and links on the diagram and marks the selected component. */
  render(layout: GraphLayout, selectedId: string): void;
  /** Releases the diagram engine; call it when the component is destroyed. */
  dispose(): void;
}

/**
 * Builds the repository map with JointJS (`@joint/core`), as `LIBS.md` assigns
 * to dependency maps: one directed node per component, one link per
 * dependency. The engine loads on demand, so the first page paint does not
 * carry it.
 */
export async function createDependencyMapDiagram(
  element: HTMLElement,
  onSelect: (id: string) => void,
): Promise<DependencyMapDiagram> {
  const joint = await import('@joint/core');
  const componentNode = defineComponentNode(joint.dia);
  const namespace = { ...joint.shapes, goconduct: { ComponentNode: componentNode } };

  const graph = new joint.dia.Graph({}, { cellNamespace: namespace });
  const paper = new joint.dia.Paper({
    el: element,
    model: graph,
    width: element.clientWidth,
    height: element.clientHeight,
    cellViewNamespace: namespace,
    // The layout is deterministic, so the nodes stay where the layout puts them.
    interactive: false,
    background: { color: 'transparent' },
  });
  paper.on('element:pointerclick', (view: dia.ElementView) => {
    const id: unknown = view.model.get('componentId');
    if (typeof id === 'string') {
      onSelect(id);
    }
  });

  let lastLayout: GraphLayout = { nodes: [], edges: [] };
  let lastSelectedId = '';

  const draw = (): void => {
    const width = element.clientWidth;
    const height = element.clientHeight;
    paper.setDimensions(width, height);
    graph.resetCells([
      ...lastLayout.edges.map((edge) => buildLink(joint.shapes, edge.relationship)),
      ...lastLayout.nodes.map((node) =>
        buildNode(componentNode, node, width, height, node.component.id === lastSelectedId),
      ),
    ]);
  };

  // The layout positions are fractions of the surface, so a resized surface
  // needs the nodes placed again.
  const observer = new ResizeObserver(() => draw());
  observer.observe(element);

  return {
    render(layout, selectedId): void {
      lastLayout = layout;
      lastSelectedId = selectedId;
      draw();
    },
    dispose(): void {
      observer.disconnect();
      paper.remove();
    },
  };
}

type ComponentNodeConstructor = ReturnType<typeof defineComponentNode>;

/** Declares the node shape: a role-colored circle, its initials, and its name below. */
function defineComponentNode(diaNamespace: typeof dia) {
  return diaNamespace.Element.define(
    'goconduct.ComponentNode',
    {
      attrs: {
        body: {
          r: 'calc(0.5*w)',
          cx: 'calc(0.5*w)',
          cy: 'calc(0.5*h)',
          stroke: DIAGRAM_COLORS.surface,
          strokeWidth: 3,
          cursor: 'pointer',
        },
        initials: {
          x: 'calc(0.5*w)',
          y: 'calc(0.5*h)',
          textAnchor: 'middle',
          textVerticalAnchor: 'middle',
          fill: DIAGRAM_COLORS.surface,
          fontSize: 11,
          fontWeight: '700',
          pointerEvents: 'none',
        },
        name: {
          x: 'calc(0.5*w)',
          y: 'calc(h+14)',
          textAnchor: 'middle',
          fill: DIAGRAM_COLORS.inkSecondary,
          fontSize: 11,
          fontWeight: '600',
          pointerEvents: 'none',
        },
      },
    },
    {
      markup: [
        { tagName: 'circle', selector: 'body' },
        { tagName: 'text', selector: 'initials' },
        { tagName: 'text', selector: 'name' },
      ],
    },
  );
}

function buildNode(
  componentNode: ComponentNodeConstructor,
  node: GraphLayout['nodes'][number],
  width: number,
  height: number,
  selected: boolean,
): dia.Element {
  return new componentNode({
    id: node.component.id,
    componentId: node.component.id,
    position: {
      x: (node.x / 100) * width - NODE_SIDE / 2,
      y: (node.y / 100) * height - NODE_SIDE / 2,
    },
    size: { width: NODE_SIDE, height: NODE_SIDE },
    z: 2,
    attrs: {
      body: {
        fill: roleColor(node.component.role),
        stroke: selected ? DIAGRAM_COLORS.focus : DIAGRAM_COLORS.surface,
      },
      initials: { text: node.component.name.slice(0, 2).toUpperCase() },
      name: { text: node.component.name },
    },
  });
}

function buildLink(
  shapes: typeof import('@joint/core').shapes,
  relationship: GraphLayout['edges'][number]['relationship'],
): dia.Link {
  return new shapes.standard.Link({
    source: { id: relationship.source },
    target: { id: relationship.target },
    z: 1,
    attrs: {
      line: {
        stroke: relationship.testOnly ? DIAGRAM_COLORS.inkTertiary : DIAGRAM_COLORS.lineStrong,
        strokeWidth: 1.5,
        strokeDasharray: relationship.testOnly ? '4 3' : 'none',
        targetMarker: { type: 'path', d: 'M 8 -4 0 0 8 4 z' },
      },
    },
  });
}
