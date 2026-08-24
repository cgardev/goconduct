import type { dia } from '@joint/core';
import { DIAGRAM_COLORS } from '../../../kernel/diagram/diagram-colors';
import type { StrategyReport } from '../../../kernel/graph/strategy-report';

const NODE_WIDTH = 200;
const NODE_HEIGHT = 52;
const COLUMN_GAP = 48;
const ROW_GAP = 72;
const CANVAS_MARGIN = 24;

/** The rendered strategy map, ready to receive a report. */
export interface StrategyMap {
  /** Replaces the layered groups and their aggregated dependencies. */
  render(report: StrategyReport): void;
  /** Releases the diagram engine; call it when the component is destroyed. */
  dispose(): void;
}

/**
 * Builds the de facto layer map with JointJS (`@joint/core`), as `LIBS.md`
 * assigns to dependency maps: one node per strategic group, placed on its
 * inferred layer, applications at the top and foundations at the bottom. A
 * healthy edge points downward; a red edge belongs to a cycle. The engine
 * loads on demand.
 */
export async function createStrategyMap(
  element: HTMLElement,
  onSelect: (groupId: string) => void,
): Promise<StrategyMap> {
  const joint = await import('@joint/core');
  const groupNode = joint.dia.Element.define(
    'goconduct.StrategyGroup',
    {
      attrs: {
        body: {
          width: 'calc(w)',
          height: 'calc(h)',
          rx: 8,
          fill: DIAGRAM_COLORS.surface,
          stroke: DIAGRAM_COLORS.lineStrong,
          strokeWidth: 1.5,
          cursor: 'pointer',
        },
        name: {
          x: 12,
          y: 20,
          fill: DIAGRAM_COLORS.ink,
          fontSize: 12,
          fontWeight: '700',
          pointerEvents: 'none',
        },
        detail: {
          x: 12,
          y: 38,
          fill: DIAGRAM_COLORS.inkTertiary,
          fontSize: 10,
          pointerEvents: 'none',
        },
      },
    },
    {
      markup: [
        { tagName: 'rect', selector: 'body' },
        { tagName: 'text', selector: 'name' },
        { tagName: 'text', selector: 'detail' },
      ],
    },
  );
  const namespace = { ...joint.shapes, goconduct: { StrategyGroup: groupNode } };

  const graph = new joint.dia.Graph({}, { cellNamespace: namespace });
  const paper = new joint.dia.Paper({
    el: element,
    model: graph,
    width: element.clientWidth,
    height: element.clientHeight,
    cellViewNamespace: namespace,
    interactive: false,
    background: { color: 'transparent' },
  });
  paper.on('element:pointerclick', (view: dia.ElementView) => {
    const id: unknown = view.model.get('groupId');
    if (typeof id === 'string') {
      onSelect(id);
    }
  });

  let lastReport: StrategyReport = { groups: [], edges: [], layers: [] };

  const draw = (): void => {
    const rows = [...lastReport.layers].reverse();
    const widest = Math.max(1, ...rows.map((row) => row.length));
    const canvasWidth = widest * (NODE_WIDTH + COLUMN_GAP) - COLUMN_GAP + 2 * CANVAS_MARGIN;
    const canvasHeight =
      Math.max(1, rows.length) * (NODE_HEIGHT + ROW_GAP) - ROW_GAP + 2 * CANVAS_MARGIN;

    const positions = new Map<string, { x: number; y: number }>();
    rows.forEach((row, rowIndex) => {
      const rowWidth = row.length * (NODE_WIDTH + COLUMN_GAP) - COLUMN_GAP;
      row.forEach((groupId, columnIndex) => {
        positions.set(groupId, {
          x: (canvasWidth - rowWidth) / 2 + columnIndex * (NODE_WIDTH + COLUMN_GAP),
          y: CANVAS_MARGIN + rowIndex * (NODE_HEIGHT + ROW_GAP),
        });
      });
    });

    // The map scales down to the surface, so every layer stays on screen.
    const scale = Math.min(1, element.clientWidth / canvasWidth);
    paper.scale(scale);
    paper.setDimensions(element.clientWidth, canvasHeight * scale);

    graph.resetCells([
      ...lastReport.edges.map((edge) =>
        buildEdge(joint.shapes, edge.source, edge.target, edge.weight, edge.cyclic),
      ),
      ...lastReport.groups.map((group) => {
        const position = positions.get(group.id) ?? { x: CANVAS_MARGIN, y: CANVAS_MARGIN };
        return new groupNode({
          id: group.id,
          groupId: group.id,
          position,
          size: { width: NODE_WIDTH, height: NODE_HEIGHT },
          z: 2,
          attrs: {
            body: {
              stroke: group.inCycle ? DIAGRAM_COLORS.danger : DIAGRAM_COLORS.lineStrong,
              strokeWidth: group.inCycle ? 2 : 1.5,
            },
            name: { text: group.id },
            detail: {
              text: `${group.components} component${group.components === 1 ? '' : 's'}`,
            },
          },
        });
      }),
    ]);
  };

  const observer = new ResizeObserver(() => draw());
  observer.observe(element);

  return {
    render(report): void {
      lastReport = report;
      draw();
    },
    dispose(): void {
      observer.disconnect();
      paper.remove();
    },
  };
}

function buildEdge(
  shapes: typeof import('@joint/core').shapes,
  source: string,
  target: string,
  weight: number,
  cyclic: boolean,
): dia.Link {
  return new shapes.standard.Link({
    source: { id: source },
    target: { id: target },
    z: 1,
    labels: [
      {
        position: 0.5,
        attrs: {
          text: {
            text: String(weight),
            fill: cyclic ? DIAGRAM_COLORS.danger : DIAGRAM_COLORS.inkSecondary,
            fontSize: 10,
            fontWeight: '600',
          },
          rect: { fill: DIAGRAM_COLORS.surface },
        },
      },
    ],
    attrs: {
      line: {
        stroke: cyclic ? DIAGRAM_COLORS.danger : DIAGRAM_COLORS.lineStrong,
        strokeWidth: Math.min(4, 1 + Math.log10(Math.max(1, weight)) * 1.5),
        strokeDasharray: cyclic ? '5 3' : 'none',
        targetMarker: { type: 'path', d: 'M 8 -4 0 0 8 4 z' },
      },
    },
  });
}
