import { provideZonelessChangeDetection } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ComponentSelectionStore } from '../../../kernel/graph/component-selection.store';
import {
  fakeComponent,
  fakeGraph,
  fakeRelationship,
  provideFakeClients,
} from '../../../testing/fake-clients';
import { ComponentDetailComponent } from './component-detail.component';

const COMPONENTS = [
  fakeComponent('internal/library/clock', 'library', 2),
  fakeComponent('cmd/goconduct', 'application', 1),
  fakeComponent('internal/kernel', 'infrastructure', 1),
];

const RELATIONSHIPS = [
  fakeRelationship('cmd/goconduct', 'internal/library/clock'),
  fakeRelationship('internal/kernel', 'internal/library/clock', true),
  fakeRelationship('internal/library/clock', 'internal/kernel'),
];

interface Rendered {
  readonly element: HTMLElement;
  readonly selection: ComponentSelectionStore;
  readonly detect: () => void;
}

async function renderPanel(): Promise<Rendered> {
  TestBed.configureTestingModule({
    providers: [
      provideZonelessChangeDetection(),
      ...(provideFakeClients(
        fakeGraph({ components: COMPONENTS, relationships: RELATIONSHIPS }),
      ) as never[]),
    ],
  });

  const harness = TestBed.createComponent(ComponentDetailComponent);
  harness.detectChanges();
  await new Promise((resolve) => setTimeout(resolve, 0));
  harness.detectChanges();

  return {
    element: harness.nativeElement as HTMLElement,
    selection: TestBed.inject(ComponentSelectionStore),
    detect: () => harness.detectChanges(),
  };
}

describe('ComponentDetailComponent', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('renders nothing until a component is selected', async () => {
    const { element } = await renderPanel();

    expect(element.querySelector('.component-detail')).toBeNull();
  });

  it('names the selected component and its metrics', async () => {
    const { element, selection, detect } = await renderPanel();

    selection.select('internal/library/clock');
    detect();

    expect(element.querySelector('#component-detail-title')?.textContent).toContain('clock');
    expect(element.querySelector('.component-detail__path')?.textContent).toContain(
      'internal/library/clock',
    );
  });

  it('leads with a plain-language reading of the metrics', async () => {
    const { element, selection, detect } = await renderPanel();

    selection.select('internal/library/clock');
    detect();

    const reading = element.querySelector('.component-detail__reading');
    expect(reading?.getAttribute('data-zone')).toBe('pain');
    expect(reading?.querySelector('.component-detail__headline')?.textContent).toContain(
      'Stable and concrete',
    );
  });

  it('lists the importers and the dependencies that produce the coupling numbers', async () => {
    const { element, selection, detect } = await renderPanel();

    selection.select('internal/library/clock');
    detect();

    const sides = element.querySelectorAll('.component-detail__side');
    expect(sides[0]?.textContent).toContain('goconduct');
    expect(sides[0]?.textContent).toContain('kernel');
    expect(sides[0]?.textContent).toContain('test only');
    expect(sides[1]?.textContent).toContain('kernel');
    expect(sides[1]?.textContent).not.toContain('goconduct');
  });

  /**
   * A reader follows a dependency by selecting it, so the drawer turns into the
   * neighbor's reading without a trip back to the table.
   */
  it('selects a neighbor from either list', async () => {
    const { element, selection, detect } = await renderPanel();
    selection.select('internal/library/clock');
    detect();

    element.querySelector<HTMLButtonElement>('.component-detail__neighbors button')?.click();
    detect();

    expect(selection.selectedComponentId()).toBe('cmd/goconduct');
  });

  it('places the component on labelled scales', async () => {
    const { element, selection, detect } = await renderPanel();

    selection.select('internal/library/clock');
    detect();

    const meters = element.querySelectorAll('[role="meter"]');
    expect(meters.length).toBe(2);
    expect(meters[0]?.getAttribute('aria-valuetext')).toContain('Stable');
    expect(meters[1]?.getAttribute('aria-valuetext')).toContain('Concrete');
  });

  /**
   * The panel is not a dialog, so nothing traps the keyboard inside it. A
   * reader who opened it from the keyboard still needs a way out that is not a
   * hunt for the close button.
   */
  it('closes on escape', async () => {
    const { element, selection, detect } = await renderPanel();
    selection.select('internal/library/clock');
    detect();

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    detect();

    expect(selection.selectedComponentId()).toBe('');
    expect(element.querySelector('.component-detail')).toBeNull();
  });

  it('closes from its own control', async () => {
    const { element, selection, detect } = await renderPanel();
    selection.select('internal/library/clock');
    detect();

    element.querySelector<HTMLButtonElement>('.component-detail__header button')?.click();
    detect();

    expect(selection.selectedComponentId()).toBe('');
  });
});
