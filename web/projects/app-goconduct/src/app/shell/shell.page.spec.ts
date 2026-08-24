import { Component, provideZonelessChangeDetection } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { fakeGraph, provideFakeClients } from '../testing/fake-clients';
import { ShellPage } from './shell.page';

/** Stands in for a routed page, so the shell has something to render into. */
@Component({
  selector: 'app-stub-page',
  template: '<p>page</p>',
})
class StubPage {}

async function renderShell(url = '/overview'): Promise<HTMLElement> {
  localStorage.clear();
  TestBed.configureTestingModule({
    providers: [
      provideZonelessChangeDetection(),
      provideRouter([
        {
          path: '',
          component: ShellPage,
          children: [
            { path: 'overview', component: StubPage },
            { path: 'components', component: StubPage },
            { path: 'findings', component: StubPage },
          ],
        },
      ]),
      ...(provideFakeClients(fakeGraph()) as never[]),
    ],
  });

  const harness = await RouterTestingHarness.create();
  await harness.navigateByUrl(url, ShellPage);
  await new Promise((resolve) => setTimeout(resolve, 0));
  harness.detectChanges();
  return harness.routeNativeElement as HTMLElement;
}

describe('ShellPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('offers one sidebar entry per page, in reading order', async () => {
    const element = await renderShell();
    const links = [...element.querySelectorAll('.shell__item')].map((item) =>
      item.textContent?.trim(),
    );

    expect(links).toEqual([
      'Overview',
      'Strategy',
      'Components',
      'Balance',
      'Findings',
      'Types',
    ]);
  });

  /**
   * The sidebar answers "where can I go". A label that navigates nowhere — the
   * release stage of the product used to sit there as a badge — belongs on the
   * page that explains it, not among the destinations.
   */
  it('keeps the sidebar to destinations only', async () => {
    const element = await renderShell();
    const sidebar = element.querySelector('.shell__sidebar');

    expect(sidebar?.textContent).not.toContain('Alpha');
    expect(sidebar?.querySelector('[tuiBadge]')).toBeNull();
    expect(sidebar?.querySelectorAll('nav button')).toHaveLength(0);
  });

  it('puts the controls that act on the whole console in the product bar', async () => {
    const element = await renderShell();
    const bar = element.querySelector('app-top-navigation');

    expect(bar?.textContent).toContain('Refresh');
    expect(bar?.querySelector('.top-navigation__status')).not.toBeNull();
    expect(bar?.querySelector('.top-navigation__source')).not.toBeNull();
  });

  it('marks exactly the open page for assistive technology', async () => {
    const element = await renderShell('/components');
    const current = element.querySelectorAll('.shell__item[aria-current="page"]');

    expect(current).toHaveLength(1);
    expect(current[0]?.textContent?.trim()).toBe('Components');
  });

  it('reports how old the graph is once the first analysis arrives', async () => {
    const element = await renderShell();

    expect(element.querySelector('.top-navigation__updated')?.textContent).toContain('Updated Now');
  });
});
