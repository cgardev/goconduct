import { provideZonelessChangeDetection } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ComponentSelectionStore } from '../../../kernel/graph/component-selection.store';
import { fakeComponent, fakeGraph, provideFakeClients } from '../../../testing/fake-clients';
import { ComponentDetailComponent } from './component-detail.component';

const COMPONENTS = [
  fakeComponent('internal/library/clock', 'library', 2),
  fakeComponent('cmd/goconduct', 'application', 1),
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
      ...(provideFakeClients(fakeGraph({ components: COMPONENTS })) as never[]),
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
