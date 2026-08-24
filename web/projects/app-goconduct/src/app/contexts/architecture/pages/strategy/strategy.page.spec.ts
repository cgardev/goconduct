import { provideZonelessChangeDetection } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import {
  fakeComponent,
  fakeGraph,
  fakeRelationship,
  provideFakeClients,
} from '../../../../testing/fake-clients';
import { StrategyPage } from './strategy.page';

const COMPONENTS = [
  fakeComponent('cmd/goconduct', 'application'),
  fakeComponent('internal/module/orders', 'application-module'),
  fakeComponent('internal/library/telemetry', 'library'),
];

const RELATIONSHIPS = [
  fakeRelationship('cmd/goconduct', 'internal/module/orders'),
  fakeRelationship('internal/module/orders', 'internal/library/telemetry'),
  fakeRelationship('internal/library/telemetry', 'internal/module/orders'),
];

// The two canvases are amCharts and JointJS, which jsdom cannot render; the
// page skips them there. The mining logic is covered by the tests of
// `strategy-report`, so this file covers the readings around the canvases.
async function renderPage(): Promise<{ element: HTMLElement; detect: () => void }> {
  TestBed.configureTestingModule({
    providers: [
      provideZonelessChangeDetection(),
      provideRouter([{ path: 'strategy', component: StrategyPage }]),
      ...(provideFakeClients(
        fakeGraph({ components: COMPONENTS, relationships: RELATIONSHIPS }),
      ) as never[]),
    ],
  });

  const harness = await RouterTestingHarness.create();
  await harness.navigateByUrl('/strategy', StrategyPage);
  await new Promise((resolve) => setTimeout(resolve, 0));
  harness.detectChanges();
  return {
    element: harness.routeNativeElement as HTMLElement,
    detect: () => harness.detectChanges(),
  };
}

describe('StrategyPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('offers one accessible control per strategic group', async () => {
    const { element } = await renderPage();

    const controls = [...element.querySelectorAll('.strategy-map__access button')].map(
      (control) => control.textContent?.replace(/\s+/g, ' ').trim(),
    );
    expect(controls).toContain('Inspect cmd/goconduct, layer 1');
  });

  it('names the cycle the strategy contradicts itself with', async () => {
    const { element } = await renderPage();

    expect(element.textContent).toContain(
      'internal/library and internal/module depend on each other.',
    );
  });

  it('lists the component dependencies behind a selected group', async () => {
    const { element, detect } = await renderPage();

    const control = [...element.querySelectorAll<HTMLButtonElement>('button')].find(
      (candidate) => candidate.textContent?.includes('Inspect cmd/goconduct'),
    );
    control?.click();
    detect();

    const rows = [...element.querySelectorAll('.strategy__relationships code')].map(
      (row) => row.textContent,
    );
    expect(rows).toEqual(['cmd/goconduct → internal/module/orders']);
  });
});
