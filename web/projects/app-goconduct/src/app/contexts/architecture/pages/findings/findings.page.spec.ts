import { provideZonelessChangeDetection } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, withComponentInputBinding } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { fakeFinding, fakeGraph, provideFakeClients } from '../../../../testing/fake-clients';
import { FindingsPage } from './findings.page';

const FINDINGS = [
  fakeFinding('no-cycles', 'error', 'internal/library/clock'),
  fakeFinding('stable-abstractions', 'warning', 'internal/module/orders'),
  fakeFinding('stable-dependencies', 'warning', 'cmd/goconduct'),
];

async function renderPage(url = '/findings', findings = FINDINGS): Promise<HTMLElement> {
  TestBed.configureTestingModule({
    providers: [
      provideZonelessChangeDetection(),
      provideRouter([{ path: 'findings', component: FindingsPage }], withComponentInputBinding()),
      ...(provideFakeClients(fakeGraph({ findings })) as never[]),
    ],
  });

  const harness = await RouterTestingHarness.create();
  await harness.navigateByUrl(url, FindingsPage);
  await new Promise((resolve) => setTimeout(resolve, 0));
  harness.detectChanges();
  return harness.routeNativeElement as HTMLElement;
}

describe('FindingsPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('lists every finding the analysis reports', async () => {
    const element = await renderPage();

    expect(element.querySelectorAll('.finding')).toHaveLength(3);
    expect(element.querySelector('.page-header__counter')?.textContent).toContain('3');
  });

  it('shows the severity from the address in the filter control', async () => {
    const element = await renderPage('/findings?severity=error');
    const trigger = element.querySelector<HTMLInputElement>('.findings__severity input');

    expect(trigger?.value).toBe('Errors');
    expect(element.querySelectorAll('.finding')).toHaveLength(1);
  });

  it('reports a satisfied graph as a result rather than as an empty list', async () => {
    const element = await renderPage('/findings', []);

    expect(element.textContent).toContain('No architecture findings');
    expect(element.textContent).not.toContain('No matches');
  });

  /**
   * A filtered list with nothing in it is a different state from a repository
   * with no findings, and it has a different way out.
   */
  it('offers a way out when the severity matches nothing', async () => {
    const element = await renderPage('/findings?severity=error', [FINDINGS[1]!]);

    expect(element.textContent).toContain('No matches');
    expect(element.textContent).toContain('Clear filter');
  });
});
