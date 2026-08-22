import { buildShellNavigation, type NavigationLink } from './shell.navigation';

/** The destination with the given identifier, searched in every group. */
function entry(id: string): NavigationLink | undefined {
  return buildShellNavigation()
    .flatMap((group) => group.children)
    .find((child) => child.id === id);
}

describe('buildShellNavigation', () => {
  it('groups every reading of the repository under architecture', () => {
    const navigation = buildShellNavigation();

    expect(navigation.map((group) => group.title)).toEqual(['Architecture']);
    expect(navigation[0]?.children.map((child) => child.title)).toEqual([
      'Overview',
      'Components',
      'Findings',
    ]);
  });

  /**
   * The link is an absolute URL string. A command array whose root segment
   * survived the join would produce '//overview', which the router matches
   * against no route at all.
   */
  it('builds an absolute link with a single leading separator', () => {
    expect(entry('overview')?.link).toBe('/overview');
    expect(entry('components')?.link).toBe('/components');
    expect(entry('findings')?.link).toBe('/findings');
  });

  it('returns an independent tree on every call so a caller cannot mutate the model', () => {
    const first = buildShellNavigation();
    const second = buildShellNavigation();

    expect(first).not.toBe(second);
    expect(first).toEqual(second);
  });
});
