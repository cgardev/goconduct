import { navTo, type NavCommands } from '../kernel/routing/app-navigation';

/** One destination in the shell sidebar. */
export interface NavigationLink {
  /** Stable identifier, used as the track key of the rendered list. */
  readonly id: string;
  /** Taiga icon name shown beside the title. */
  readonly icon: string;
  /** Text of the link. */
  readonly title: string;
  /** Absolute URL the link navigates to. */
  readonly link: string;
}

/** One titled group of destinations in the shell sidebar. */
export interface NavigationGroup {
  /** Stable identifier, used as the track key of the rendered list. */
  readonly id: string;
  /** Heading of the group. */
  readonly title: string;
  /** One line that states what the group answers. */
  readonly subtitle: string;
  /** Destinations of the group, in reading order. */
  readonly children: readonly NavigationLink[];
}

// toLink flattens an absolute command array into the URL string the navigation
// item binds to `routerLink`. Every command array starts with the root '/',
// which must not survive as a segment, or the join would produce '//overview'.
function toLink(commands: NavCommands): string {
  return `/${commands.filter((segment) => segment !== '/').join('/')}`;
}

/**
 * Navigation model of the shell sidebar. The tree is built from the typed
 * navigation helpers rather than from hand-written paths, so a route change
 * reaches this file without an edit.
 *
 * Pages add their entries here as they land, so this file stays the one place
 * the menu grows.
 */
export function buildShellNavigation(): NavigationGroup[] {
  return [
    {
      id: 'architecture',
      title: 'ARCHITECTURE',
      subtitle: 'Readings of the analyzed repository',
      children: [
        {
          id: 'overview',
          icon: '@tui.layout-dashboard',
          title: 'Overview',
          link: toLink(navTo.overview()),
        },
        {
          id: 'components',
          icon: '@tui.boxes',
          title: 'Components',
          link: toLink(navTo.components()),
        },
        {
          id: 'findings',
          icon: '@tui.list-checks',
          title: 'Findings',
          link: toLink(navTo.findings()),
        },
      ],
    },
  ];
}
