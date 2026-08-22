import { provideZonelessChangeDetection } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { fakeGraph, provideFakeClients } from '../testing/fake-clients';
import { ShellPage } from './shell.page';

async function renderShell(): Promise<HTMLElement> {
  localStorage.clear();
  TestBed.configureTestingModule({
    providers: [
      provideZonelessChangeDetection(),
      provideRouter([{ path: '', component: ShellPage }]),
      ...(provideFakeClients(fakeGraph()) as never[]),
    ],
  });

  const harness = await RouterTestingHarness.create();
  await harness.navigateByUrl('/', ShellPage);
  await new Promise((resolve) => setTimeout(resolve, 0));
  harness.detectChanges();
  return harness.routeNativeElement as HTMLElement;
}

describe('ShellPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('offers one sidebar entry per page, in reading order', async () => {
    const element = await renderShell();
    const links = [...element.querySelectorAll('.shell__item')].map((item) => item.textContent);

    expect(links.map((text) => text?.trim())).toEqual(['Overview', 'Components', 'Findings']);
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
    // Every interactive element of the navigation is a link.
    expect(sidebar?.querySelectorAll('nav button')).toHaveLength(0);
  });

  it('puts the controls that act on the whole console in the product bar', async () => {
    const element = await renderShell();
    const bar = element.querySelector('app-top-navigation');

    expect(bar?.textContent).toContain('Refresh');
    expect(bar?.querySelector('.top-navigation__status')).not.toBeNull();
    expect(bar?.querySelector('.top-navigation__source')).not.toBeNull();
  });

  it('marks the open page for assistive technology', async () => {
    const element = await renderShell();

    expect(element.querySelectorAll('.shell__item[aria-current="page"]').length).toBeLessThan(2);
  });
});
