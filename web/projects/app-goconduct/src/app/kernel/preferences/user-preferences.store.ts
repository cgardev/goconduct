import { Service, signal } from '@angular/core';

// Preference keys are dot-namespaced, lowercase identifiers, and their values
// are always JSON. Every key is stored under this prefix, so an unrelated
// localStorage entry of the origin never collides with a console preference.
const STORAGE_PREFIX = 'goconduct.preferences.';

/**
 * Persists the reader's interface preferences, such as the sidebar density, in
 * the browser's localStorage. They are presentation choices with no server-side
 * counterpart, so they stay local to the device. The store mirrors the stored
 * entries into a signal, so every consumer recomputes when a preference changes.
 */
@Service()
export class UserPreferencesStore {
  private readonly entries = signal<ReadonlyMap<string, string>>(loadEntries());

  /**
   * Returns the JSON value stored under `key`, parsed as `T`, or null when the
   * key is absent or its payload does not parse. The read is reactive: a read
   * inside a computed or an effect re-runs when the preferences change.
   */
  read<T>(key: string): T | null {
    const raw = this.entries().get(key);
    if (raw === undefined) {
      return null;
    }
    try {
      return JSON.parse(raw) as T;
    } catch {
      return null;
    }
  }

  /** Serializes `value` to JSON and persists it under `key`. */
  write(key: string, value: unknown): void {
    const serialized = JSON.stringify(value);
    const next = new Map(this.entries());
    next.set(key, serialized);
    this.entries.set(next);
    try {
      localStorage.setItem(STORAGE_PREFIX + key, serialized);
    } catch (cause: unknown) {
      // Quota exhaustion, or a privacy mode that blocks storage: the in-memory
      // value still applies for the session, so the failure is only logged.
      console.error(`failed to persist preference "${key}"`, cause);
    }
  }
}

// loadEntries reads every prefixed preference from localStorage into the initial
// map, so a read is reactive from the first render without an asynchronous load.
function loadEntries(): ReadonlyMap<string, string> {
  const entries = new Map<string, string>();
  try {
    for (let index = 0; index < localStorage.length; index++) {
      const storageKey = localStorage.key(index);
      if (storageKey === null || !storageKey.startsWith(STORAGE_PREFIX)) {
        continue;
      }
      const value = localStorage.getItem(storageKey);
      if (value !== null) {
        entries.set(storageKey.slice(STORAGE_PREFIX.length), value);
      }
    }
  } catch (cause: unknown) {
    console.error('failed to load preferences from localStorage', cause);
  }
  return entries;
}
