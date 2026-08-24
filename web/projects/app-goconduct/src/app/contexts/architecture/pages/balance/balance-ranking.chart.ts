import { registerAmChartsLicense } from '../../../../kernel/diagram/amcharts-license';
import { DIAGRAM_COLORS } from '../../../../kernel/diagram/diagram-colors';
import type { BalanceEntry } from '../../../../kernel/graph/balance-report';

/** The rendered ranking, ready to receive entries. */
export interface BalanceRankingChart {
  /** Replaces the ranked bars. */
  render(entries: readonly BalanceEntry[]): void;
  /** Releases the chart engine; call it when the component is destroyed. */
  dispose(): void;
}

/**
 * Builds the balance ranking with amCharts 5: one horizontal bar per
 * component, the riskiest on top, the bar as long as its number of
 * dependents. `LIBS.md` assigns bars to the comparison of metrics between
 * components. The engine loads on demand.
 */
export async function createBalanceRankingChart(
  element: HTMLElement,
  onSelect: (id: string) => void,
): Promise<BalanceRankingChart> {
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
      // Room for the last axis label, so nothing runs past the right edge.
      paddingRight: 24,
    }),
  );

  const yAxis = chart.yAxes.push(
    am5xy.CategoryAxis.new(root, {
      categoryField: 'label',
      renderer: am5xy.AxisRendererY.new(root, {
        inversed: true,
        minGridDistance: 12,
        cellStartLocation: 0.15,
        cellEndLocation: 0.85,
      }),
    }),
  );
  const xAxis = chart.xAxes.push(
    am5xy.ValueAxis.new(root, {
      min: 0,
      maxPrecision: 0,
      // The longest bar keeps a gap to the plot edge.
      extraMax: 0.06,
      renderer: am5xy.AxisRendererX.new(root, {}),
    }),
  );
  xAxis.children.push(
    am5.Label.new(root, {
      text: 'Components that depend on it →',
      x: am5.p50,
      centerX: am5.p50,
      fontSize: 12,
      fill: am5.color(DIAGRAM_COLORS.inkSecondary),
    }),
  );

  const series = chart.series.push(
    am5xy.ColumnSeries.new(root, {
      xAxis,
      yAxis,
      valueXField: 'dependents',
      categoryYField: 'label',
    }),
  );
  series.columns.template.setAll({
    cornerRadiusTR: 4,
    cornerRadiusBR: 4,
    strokeOpacity: 0,
    cursorOverStyle: 'pointer',
    tooltipText: '{summary}',
    focusable: true,
    role: 'button',
    ariaLabel: '{summary}',
  });
  series.columns.template.adapters.add('fill', (fill, target) => {
    const entry = target.dataItem?.dataContext as BalanceEntry | undefined;
    return entry === undefined ? fill : am5.color(entry.color);
  });
  series.columns.template.events.on('click', (event) => {
    const entry = event.target.dataItem?.dataContext as BalanceEntry | undefined;
    if (entry !== undefined) {
      onSelect(entry.id);
    }
  });

  return {
    render(entries): void {
      yAxis.data.setAll([...entries]);
      series.data.setAll([...entries]);
    },
    dispose(): void {
      root.dispose();
    },
  };
}
