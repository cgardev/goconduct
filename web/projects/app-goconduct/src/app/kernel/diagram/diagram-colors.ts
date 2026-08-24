/**
 * Colors of the diagrams that paint outside the stylesheet.
 *
 * amCharts renders on a canvas and JointJS writes its own SVG attributes, so
 * neither can read the LESS variables of `styles/tokens.less`. This module
 * mirrors the tokens the diagrams need. Keep both files in step when a color
 * changes.
 */
export const DIAGRAM_COLORS = {
  surface: '#ffffff',
  ink: '#0d0d0d',
  inkSecondary: '#6e6e80',
  inkTertiary: '#8e8ea0',
  line: '#e5e5e5',
  lineStrong: '#d1d1d6',
  accent: '#10a37f',
  accentDark: '#0d8a6b',
  danger: '#c2352a',
  dangerSoft: '#fff4f2',
  warning: '#a65f00',
  warningSoft: '#fff8e6',
  focus: '#0b74de',
} as const;

/** One color per component role, mirroring the `role-colors` mixin. */
const ROLE_COLORS: Readonly<Record<string, string>> = {
  'application': '#3f6f8f',
  'application-module': '#5d5d8f',
  'shared-module': '#287d68',
  'library': '#6e6e80',
  'infrastructure': '#936426',
  'development': '#8a5578',
};

/** Returns the color of one component role, with the library color as the fallback. */
export function roleColor(role: string): string {
  return ROLE_COLORS[role] ?? ROLE_COLORS['library'];
}

/** One color per Go type kind, mirroring the `type-*` tokens of `tokens.less`. */
const TYPE_KIND_COLORS: Readonly<Record<string, string>> = {
  'struct': '#3f6f8f',
  'interface': '#287d68',
  'alias': '#8a5578',
  'basic': '#936426',
  'external': '#8e8ea0',
};

/** Returns the color of one Go type kind, with the external color as the fallback. */
export function typeKindColor(kind: string): string {
  return TYPE_KIND_COLORS[kind] ?? TYPE_KIND_COLORS['external'];
}
