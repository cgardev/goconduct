import type { Color } from '@amcharts/amcharts5';
import { registerAmChartsLicense } from '../../../../kernel/diagram/amcharts-license';
import { DIAGRAM_COLORS } from '../../../../kernel/diagram/diagram-colors';
import type { StrategyReport } from '../../../../kernel/graph/strategy-report';

/** One drawn cell of the dependency matrix. */
interface MatrixCell {
  readonly source: string;
  readonly target: string;
  readonly weight: number;
  readonly summary: string;
  readonly cyclic: boolean;
  readonly columnSettings: { readonly fill: Color };
  readonly labelColor: Color;
}

/** The rendered matrix, ready to receive a report. */
export interface StrategyMatrixChart {
  /** Replaces the matrix cells. */
  render(report: StrategyReport): void;
  /** Releases the chart engine; call it when the component is destroyed. */
  dispose(): void;
}

/**
 * Builds the group dependency matrix with amCharts 5: a heat map, as
 * `LIBS.md` assigns to numeric intensity between two dimensions. The row
 * group depends on the column group; the cell carries the aggregated count,
 * and a red cell belongs to a cycle. The engine loads on demand.
 */
export async function createStrategyMatrix(
  element: HTMLElement,
  onSelect: (source: string, target: string) => void,
): Promise<StrategyMatrixChart> {
  const am5 = await import('@amcharts/amcharts5');
  const am5xy = await import('@amcharts/amcharts5/xy');
  const am5themes_Animated = (await import('@amcharts/amcharts5/themes/Animated')).default;
  registerAmChartsLicense(am5);

  const root = am5.Root.new(element);
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  root.setThemes(reducedMotion ? [] : [am5themes_Animated.new(root)]);

  const chart = root.container.children.push(
    am5xy.XYChart.new(root, {
      panX: false,
      panY: false,
      wheelX: 'none',
      wheelY: 'none',
      paddingLeft: 0,
      paddingRight: 24,
    }),
  );

  const yRenderer = am5xy.AxisRendererY.new(root, { minGridDistance: 16, inversed: true });
  yRenderer.grid.template.set('visible', false);
  const yAxis = chart.yAxes.push(
    am5xy.CategoryAxis.new(root, { categoryField: 'group', renderer: yRenderer }),
  );
  yAxis.children.unshift(
    am5.Label.new(root, {
      text: 'Depends on →',
      rotation: -90,
      y: am5.p50,
      centerX: am5.p50,
      fontSize: 12,
      fill: am5.color(DIAGRAM_COLORS.inkSecondary),
    }),
  );

  const xRenderer = am5xy.AxisRendererX.new(root, { minGridDistance: 24 });
  xRenderer.grid.template.set('visible', false);
  xRenderer.labels.template.setAll({
    rotation: -40,
    centerY: am5.p50,
    centerX: am5.p100,
    fontSize: 11,
  });
  const xAxis = chart.xAxes.push(
    am5xy.CategoryAxis.new(root, { categoryField: 'group', renderer: xRenderer }),
  );

  const series = chart.series.push(
    am5xy.ColumnSeries.new(root, {
      xAxis,
      yAxis,
      categoryXField: 'target',
      categoryYField: 'source',
      valueField: 'weight',
      clustered: false,
      stroke: am5.color(DIAGRAM_COLORS.surface),
    }),
  );
  series.columns.template.setAll({
    width: am5.percent(100),
    height: am5.percent(100),
    strokeOpacity: 1,
    strokeWidth: 2,
    cornerRadiusTL: 4,
    cornerRadiusTR: 4,
    cornerRadiusBL: 4,
    cornerRadiusBR: 4,
    cursorOverStyle: 'pointer',
    tooltipText: '{summary}',
    focusable: true,
    role: 'button',
    ariaLabel: '{summary}',
    templateField: 'columnSettings',
  });
  series.columns.template.events.on('click', (event) => {
    const cell = event.target.dataItem?.dataContext as MatrixCell | undefined;
    if (cell !== undefined) {
      onSelect(cell.source, cell.target);
    }
  });
  series.bullets.push((_bulletRoot, _series, dataItem) => {
    const cell = dataItem.dataContext as MatrixCell;
    return am5.Bullet.new(root, {
      sprite: am5.Label.new(root, {
        text: String(cell.weight),
        fill: cell.labelColor,
        fontSize: 10,
        fontWeight: '600',
        centerX: am5.p50,
        centerY: am5.p50,
      }),
    });
  });

  return {
    render(report): void {
      // The rows and the columns share one order: the highest layer first,
      // so a healthy matrix concentrates below its diagonal.
      const order = [...report.groups]
        .sort((first, second) => second.layer - first.layer || first.id.localeCompare(second.id))
        .map((group) => ({ group: group.id }));
      const heaviest = Math.max(1, ...report.edges.map((edge) => edge.weight));
      const cells = report.edges.map((edge): MatrixCell => {
        const intensity = edge.weight / heaviest;
        return {
          source: edge.source,
          target: edge.target,
          weight: edge.weight,
          summary:
            `${edge.source} depends on ${edge.target} through ${edge.weight} ` +
            `component dependenc${edge.weight === 1 ? 'y' : 'ies'}` +
            `${edge.cyclic ? '. The two groups form a cycle.' : '.'}`,
          cyclic: edge.cyclic,
          columnSettings: {
            fill: edge.cyclic
              ? am5.color(DIAGRAM_COLORS.danger)
              : am5.Color.lighten(am5.color(DIAGRAM_COLORS.accent), 0.75 - 0.75 * intensity),
          },
          labelColor: am5.color(
            edge.cyclic || intensity > 0.45 ? DIAGRAM_COLORS.surface : DIAGRAM_COLORS.ink,
          ),
        };
      });
      yAxis.data.setAll([...order]);
      xAxis.data.setAll([...order]);
      series.data.setAll(cells);
    },
    dispose(): void {
      root.dispose();
    },
  };
}
