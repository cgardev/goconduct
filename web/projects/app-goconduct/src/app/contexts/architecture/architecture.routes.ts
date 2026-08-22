import { Routes } from '@angular/router';
import { ROUTE_PATH } from '../../kernel/routing/route-paths';

/** Routes that mount the readings of the analyzed repository. */
export const ARCHITECTURE_ROUTES: Routes = [
  {
    path: ROUTE_PATH.overview,
    loadComponent: () =>
      import('./pages/overview/overview.page').then((module) => module.OverviewPage),
  },
  {
    path: ROUTE_PATH.components,
    loadComponent: () =>
      import('./pages/components/components.page').then((module) => module.ComponentsPage),
  },
  {
    path: ROUTE_PATH.findings,
    loadComponent: () =>
      import('./pages/findings/findings.page').then((module) => module.FindingsPage),
  },
];
