import { provideZonelessChangeDetection } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { create } from '@bufbuild/protobuf';
import { ComponentSchema, type Component as GraphComponent } from 'lib-api-gen/gen/v1/graph_pb';
import { fakeGraph, provideFakeClients } from '../../../../testing/fake-clients';
import { BalancePage } from './balance.page';

const PAINFUL: GraphComponent = create(ComponentSchema, {
  id: 'pkg/failure',
  name: 'failure',
  role: 'library',
  afferentCoupling: 17,
  instability: 0,
  abstractness: 0,
  mainSequenceDistance: 1,
});
const BALANCED: GraphComponent = create(ComponentSchema, {
  id: 'pkg/report',
  name: 'report',
  role: 'library',
  afferentCoupling: 2,
  efferentCoupling: 2,
  instability: 0.5,
  abstractness: 0.4,
  mainSequenceDistance: 0.1,
});

// The two charts are amCharts on a canvas, which jsdom cannot render; the page
// skips them there. The ranking and bucket logic is covered by the tests of
// `balance-report`, so this file covers the page around the charts.
async function renderPage(components: GraphComponent[] = [PAINFUL, BALANCED]): Promise<HTMLElement> {
  TestBed.configureTestingModule({
    providers: [
      provideZonelessChangeDetection(),
      provideRouter([{ path: 'balance', component: BalancePage }]),
      ...(provideFakeClients(fakeGraph({ components })) as never[]),
    ],
  });

  const harness = await RouterTestingHarness.create();
  await harness.navigateByUrl('/balance', BalancePage);
  await new Promise((resolve) => setTimeout(resolve, 0));
  harness.detectChanges();
  return harness.routeNativeElement as HTMLElement;
}

describe('BalancePage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('counts the components of every zone', async () => {
    const element = await renderPage();
    const cards = [...element.querySelectorAll('.zone-card')];

    expect(cards).toHaveLength(6);
    const pain = cards.find((card) => card.getAttribute('data-zone') === 'pain');
    expect(pain?.querySelector('.zone-card__count')?.textContent?.trim()).toBe('1');
    expect(pain?.textContent).toContain('Zone of pain');
  });

  it('offers the ranking with an accessible description', async () => {
    const element = await renderPage();
    const surface = element.querySelector('.balance__ranking');

    expect(surface?.getAttribute('aria-label')).toContain('one bar per component');
  });

  it('reports that nothing calls for attention when every component is balanced', async () => {
    const element = await renderPage([BALANCED]);

    expect(element.textContent).toContain('Nothing calls for attention');
    expect(element.querySelector('.balance__ranking')).toBeNull();
  });

});
