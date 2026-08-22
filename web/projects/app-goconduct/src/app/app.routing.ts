import { Routes } from '@angular/router';
import { ROUTE_PATH } from './kernel/routing/route-paths';

/**
 * Top-level routes.
 *
 * Everything hangs off the shell, which therefore renders exactly once and
 * survives every navigation. Pages are mounted as lazily loaded children, one
 * group per context.
 */
export const appRoutes: Routes = [
  {
    path: ROUTE_PATH.empty,
    loadComponent: () => import('./shell/shell.page').then((module) => module.ShellPage),
    children: [
      { path: ROUTE_PATH.empty, pathMatch: 'full', redirectTo: ROUTE_PATH.overview },
      {
        path: ROUTE_PATH.empty,
        loadChildren: () =>
          import('./contexts/architecture/architecture.routes').then(
            (module) => module.ARCHITECTURE_ROUTES,
          ),
      },
      {
        path: ROUTE_PATH.wildcard,
        loadComponent: () =>
          import('./shell/error-404/error-404.page').then((module) => module.Error404Page),
      },
    ],
  },
];
