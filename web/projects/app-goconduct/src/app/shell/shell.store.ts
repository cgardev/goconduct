import { computed, inject, Service } from '@angular/core';
import { UserPreferencesStore } from '../kernel/preferences/user-preferences.store';

/** Preference key the sidebar density is persisted under, as JSON. */
const SIDEBAR_DENSITY_KEY = 'appearance.sidebarDensity';

/** Shape persisted under {@link SIDEBAR_DENSITY_KEY}. */
interface SidebarDensityPreference {
  readonly dense: boolean;
}

/**
 * Presentation state of the application shell. The collapsed choice lives here
 * rather than in the sidebar's own component, so the preferences store persists
 * it and it survives a reload.
 */
@Service()
export class ShellStore {
  private readonly preferences = inject(UserPreferencesStore);

  /** Whether the sidebar is collapsed to its icon rail. */
  readonly dense = computed(
    () => this.preferences.read<SidebarDensityPreference>(SIDEBAR_DENSITY_KEY)?.dense === true,
  );

  /** Collapses an expanded sidebar and expands a collapsed one. */
  toggleDensity(): void {
    this.preferences.write(SIDEBAR_DENSITY_KEY, {
      dense: !this.dense(),
    } satisfies SidebarDensityPreference);
  }
}
