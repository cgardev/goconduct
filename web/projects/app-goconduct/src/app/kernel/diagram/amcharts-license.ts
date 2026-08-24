/**
 * License key of amCharts 5.
 *
 * The free version of amCharts prints its logo on every chart, and its terms
 * only allow the logo to go away with a commercial license. Paste the key
 * (`AM5C...`) here; every chart registers it before it builds and the logo
 * disappears. An empty key keeps the free version with its logo.
 */
export const AMCHARTS_LICENSE_KEY = '';

/** Registers the license once, before the first chart root is created. */
export function registerAmChartsLicense(am5: typeof import('@amcharts/amcharts5')): void {
  if (AMCHARTS_LICENSE_KEY !== '') {
    am5.addLicense(AMCHARTS_LICENSE_KEY);
  }
}
