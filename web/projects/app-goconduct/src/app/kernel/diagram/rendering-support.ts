/**
 * Capability checks for the diagram engines.
 *
 * The unit tests run under jsdom, which implements neither the canvas 2D
 * context amCharts draws on nor the SVG geometry API JointJS measures with.
 * Each diagram component checks the capability it needs and keeps its plain
 * markup when the environment cannot render the diagram.
 */

/** Whether the environment offers the canvas 2D context amCharts needs. */
export function supportsCanvasRendering(): boolean {
  // The global type is checked first because jsdom logs a warning when the
  // context is requested; a browser that blocks the context still returns null.
  return (
    typeof CanvasRenderingContext2D === 'function' &&
    document.createElement('canvas').getContext('2d') !== null
  );
}

/** Whether the environment offers the SVG geometry API JointJS needs. */
export function supportsSvgGeometry(): boolean {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  return typeof svg.createSVGPoint === 'function';
}
