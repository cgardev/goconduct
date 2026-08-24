import { registerAmChartsLicense } from '../../../../kernel/diagram/amcharts-license';
import { DIAGRAM_COLORS } from '../../../../kernel/diagram/diagram-colors';
import type { DistanceBucket } from '../../../../kernel/graph/balance-report';

/** The rendered distribution, ready to receive buckets. */
export interface BalanceDistributionChart {
  /** Replaces the histogram buckets. */
  render(buckets: readonly DistanceBucket[]): void;
  /** Releases the chart engine; call it when the component is destroyed. */
  dispose(): void;
}

/**
 * Builds the distance distribution with amCharts 5: a histogram over the
 * distance from the main sequence, as `LIBS.md` assigns to distributions. The
 * engine loads on demand.
 */
export async function createBalanceDistributionChart(
  element: HTMLElement,
): Promise<BalanceDistributionChart> {
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

  const xAxis = chart.xAxes.push(
    am5xy.CategoryAxis.new(root, {
      categoryField: 'label',
      renderer: am5xy.AxisRendererX.new(root, { minGridDistance: 24 }),
    }),
  );
  xAxis.children.push(
    am5.Label.new(root, {
      text: 'Distance from the balance →',
      x: am5.p50,
      centerX: am5.p50,
      fontSize: 12,
      fill: am5.color(DIAGRAM_COLORS.inkSecondary),
    }),
  );
  const yAxis = chart.yAxes.push(
    am5xy.ValueAxis.new(root, {
      min: 0,
      maxPrecision: 0,
      renderer: am5xy.AxisRendererY.new(root, {}),
    }),
  );
  yAxis.children.unshift(
    am5.Label.new(root, {
      text: 'Components',
      rotation: -90,
      y: am5.p50,
      centerX: am5.p50,
      fontSize: 12,
      fill: am5.color(DIAGRAM_COLORS.inkSecondary),
    }),
  );

  const series = chart.series.push(
    am5xy.ColumnSeries.new(root, {
      xAxis,
      yAxis,
      valueYField: 'count',
      categoryXField: 'label',
    }),
  );
  series.columns.template.setAll({
    cornerRadiusTL: 4,
    cornerRadiusTR: 4,
    strokeOpacity: 0,
    tooltipText: '{summary}',
    focusable: true,
    ariaLabel: '{summary}',
  });
  series.columns.template.adapters.add('fill', (fill, target) => {
    const bucket = target.dataItem?.dataContext as DistanceBucket | undefined;
    return bucket === undefined ? fill : am5.color(bucket.color);
  });

  return {
    render(buckets): void {
      xAxis.data.setAll([...buckets]);
      series.data.setAll([...buckets]);
    },
    dispose(): void {
      root.dispose();
    },
  };
}
