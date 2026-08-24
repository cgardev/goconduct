import { provideZonelessChangeDetection } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { create } from '@bufbuild/protobuf';
import { GraphSummarySchema } from 'lib-api-gen/gen/v1/graph_pb';
import { fakeComponent, fakeGraph, provideFakeClients } from '../../../../testing/fake-clients';
import { OverviewPage } from './overview.page';

const SUMMARY = create(GraphSummarySchema, {
  components: 3,
  applications: 1,
  productionRelationships: 4,
  findings: 2,
  errors: 1,
  warnings: 1,
});

async function renderPage(plugins: string[] = ['coverage']): Promise<HTMLElement> {
  TestBed.configureTestingModule({
    providers: [
      provideZonelessChangeDetection(),
      provideRouter([{ path: 'overview', component: OverviewPage }]),
      ...(provideFakeClients(
        fakeGraph({
          summary: SUMMARY,
          components: [
            fakeComponent('internal/library/clock', 'library', 2),
            fakeComponent('cmd/goconduct', 'application', 1),
          ],
        }),
        plugins,
      ) as never[]),
    ],
  });

  const harness = await RouterTestingHarness.create();
  await harness.navigateByUrl('/overview', OverviewPage);
  await new Promise((resolve) => setTimeout(resolve, 0));
  harness.detectChanges();
  return harness.routeNativeElement as HTMLElement;
}

describe('OverviewPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  /**
   * The page reports state, so it opens with a heading rather than with the
   * full-height banner it used to carry. A reader who arrives to check whether
   * anything changed should not have to scroll past a sales line first.
   */
  it('opens with the page heading rather than with a banner', async () => {
    const element = await renderPage();
    const heading = element.querySelector('h1');

    expect(heading?.textContent?.trim()).toBe('Overview');
  });

  it('states the release stage on the page the reader lands on', async () => {
    const element = await renderPage();

    expect(element.querySelector('app-alert')?.textContent).toContain('alpha');
  });

  it('leads from the summary to the page that details it', async () => {
    const element = await renderPage();
    const links = [...element.querySelectorAll('a.metric-card')].map((card) =>
      card.getAttribute('href'),
    );

    expect(links).toEqual(['/components', '/findings']);
  });

  it('places one node per mapped component', async () => {
    const element = await renderPage();

    expect(element.querySelectorAll('.dependency-map__access button')).toHaveLength(2);
  });

  /**
   * The plugin catalog feeds one card. Its failure used to be reported through
   * the same message as a failed analysis, which claimed the whole graph was
   * unavailable while a complete graph was on screen.
   */
  it('keeps a missing plugin catalog out of the analysis state', async () => {
    const element = await renderPage([]);

    expect(element.querySelector('app-dependency-map')).not.toBeNull();
    expect(element.textContent).not.toContain('The analysis is unavailable');
  });
});
