import { provideZonelessChangeDetection } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter, withComponentInputBinding } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { ComponentTypesStore } from '../../../../kernel/graph/component-types.store';
import {
  fakeComponent,
  fakeGraph,
  fakeTypeDeclaration,
  provideFakeClients,
} from '../../../../testing/fake-clients';
import { TypesPage } from './types.page';

const COMPONENTS = [
  fakeComponent('internal/library/telemetry', 'library', 2),
  fakeComponent('internal/module/orders', 'application-module', 1),
];

const TYPES = [
  fakeTypeDeclaration('internal/module/orders.Order', 'struct', {
    component: 'internal/module/orders',
    fields: [{ name: 'Sink', type: 'telemetry.Writer', embedded: false, exported: true }],
    implements: [
      { id: 'internal/library/telemetry.Writer', component: 'internal/library/telemetry' },
    ],
  }),
  fakeTypeDeclaration('internal/module/orders.Line', 'struct', {
    component: 'internal/module/orders',
  }),
];

interface Rendered {
  readonly element: HTMLElement;
  readonly harness: RouterTestingHarness;
}

async function renderPage(url = '/types'): Promise<Rendered> {
  TestBed.configureTestingModule({
    providers: [
      provideZonelessChangeDetection(),
      provideRouter([{ path: 'types', component: TypesPage }], withComponentInputBinding()),
      ...(provideFakeClients(fakeGraph({ components: COMPONENTS }), [], TYPES) as never[]),
    ],
  });

  const harness = await RouterTestingHarness.create();
  await harness.navigateByUrl(url, TypesPage);
  // The stores resolve their responses in promises, so the rendered page only
  // holds the diagram after the microtask queue drains.
  await new Promise((resolve) => setTimeout(resolve, 0));
  harness.detectChanges();
  await new Promise((resolve) => setTimeout(resolve, 0));
  harness.detectChanges();
  return { element: harness.routeNativeElement as HTMLElement, harness };
}

describe('TypesPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  // The first render of the file pays the cold import of the tree widget,
  // so this test carries a wider timeout than the default.
  it('asks the reader to pick a component before drawing anything', async () => {
    const { element } = await renderPage();

    expect(element.textContent).toContain('Pick a component');
    expect(element.querySelector('app-type-diagram')).toBeNull();
  }, 15000);

  it('loads the component the address names and diagrams its types', async () => {
    const { element } = await renderPage('/types?component=internal%2Fmodule%2Forders');

    const store = TestBed.inject(ComponentTypesStore);
    expect(store.componentId()).toBe('internal/module/orders');
    const controls = element.querySelectorAll('.type-diagram__access button');
    expect(controls.length).toBeGreaterThan(0);
    expect(element.textContent).toContain('Inspect Order');
  });

  it('marks the picked component in the component tree', async () => {
    const { element } = await renderPage('/types?component=internal%2Fmodule%2Forders');

    const selected = element.querySelector('.component-tree__item--selected');
    expect(selected?.textContent?.trim()).toContain('orders');
    expect(selected?.getAttribute('aria-pressed')).toBe('true');
  });

  it('opens the detail panel on the selected type', async () => {
    const { element, harness } = await renderPage('/types?component=internal%2Fmodule%2Forders');

    const store = TestBed.inject(ComponentTypesStore);
    store.selectType('internal/module/orders.Order');
    harness.detectChanges();

    const panel = element.querySelector('.type-detail');
    expect(panel?.textContent).toContain('Order');
    expect(panel?.textContent).toContain('«struct»');
    expect(panel?.textContent).toContain('Sink: telemetry.Writer');
  });

  it('names the other component beside a cross-component relation', async () => {
    const { element, harness } = await renderPage('/types?component=internal%2Fmodule%2Forders');

    TestBed.inject(ComponentTypesStore).selectType('internal/module/orders.Order');
    harness.detectChanges();

    const relations = element.querySelectorAll('.type-detail__relations button');
    const implementsControl = [...relations].find((control) =>
      control.textContent?.includes('Writer'),
    );
    expect(implementsControl?.textContent).toContain('internal/library/telemetry');
  });

  it('navigates to the other component when the reader follows the relation', async () => {
    const { element, harness } = await renderPage('/types?component=internal%2Fmodule%2Forders');

    TestBed.inject(ComponentTypesStore).selectType('internal/module/orders.Order');
    harness.detectChanges();

    const relations = element.querySelectorAll<HTMLButtonElement>('.type-detail__relations button');
    [...relations].find((control) => control.textContent?.includes('Writer'))?.click();
    await new Promise((resolve) => setTimeout(resolve, 0));
    harness.detectChanges();

    expect(TestBed.inject(ComponentTypesStore).componentId()).toBe('internal/library/telemetry');
  });

  it('reports an empty component instead of an empty canvas', async () => {
    const { element } = await renderPage('/types?component=internal%2Flibrary%2Ftelemetry');

    expect(element.textContent).toContain('No types');
    expect(element.querySelector('app-type-diagram')).toBeNull();
  });
});
