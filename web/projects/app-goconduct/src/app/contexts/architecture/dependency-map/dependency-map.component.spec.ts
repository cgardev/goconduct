import { Component, provideZonelessChangeDetection } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ComponentSelectionStore } from '../../../kernel/graph/component-selection.store';
import { buildGraphLayout, type GraphLayout } from '../../../kernel/graph/graph-layout';
import {
  fakeComponent,
  fakeGraph,
  fakeRelationship,
  provideFakeClients,
} from '../../../testing/fake-clients';
import { DependencyMapComponent } from './dependency-map.component';

const COMPONENTS = [
  fakeComponent('internal/library/clock', 'library', 2),
  fakeComponent('cmd/goconduct', 'application', 1),
];

const LAYOUT = buildGraphLayout(COMPONENTS, [
  fakeRelationship('cmd/goconduct', 'internal/library/clock'),
]);

@Component({
  imports: [DependencyMapComponent],
  template: '<app-dependency-map [layout]="layout" />',
})
class HostComponent {
  readonly layout: GraphLayout = LAYOUT;
}

interface Rendered {
  readonly element: HTMLElement;
  readonly selection: ComponentSelectionStore;
  readonly detect: () => void;
}

// The diagram itself is JointJS over the SVG geometry API, which jsdom does
// not implement; the component skips it there. This file covers the hidden
// access list, which carries the same selection for the keyboard.
async function renderMap(): Promise<Rendered> {
  TestBed.configureTestingModule({
    providers: [
      provideZonelessChangeDetection(),
      ...(provideFakeClients(fakeGraph({ components: [...COMPONENTS] })) as never[]),
    ],
  });

  const harness = TestBed.createComponent(HostComponent);
  harness.detectChanges();
  await new Promise((resolve) => setTimeout(resolve, 0));
  harness.detectChanges();

  return {
    element: harness.nativeElement as HTMLElement,
    selection: TestBed.inject(ComponentSelectionStore),
    detect: () => harness.detectChanges(),
  };
}

describe('DependencyMapComponent', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('offers one accessible control per mapped component', async () => {
    const { element } = await renderMap();

    const controls = element.querySelectorAll('.dependency-map__access button');
    expect(controls).toHaveLength(2);
    expect(controls[0]?.textContent).toContain('Inspect');
  });

  it('selects the component behind a control', async () => {
    const { element, selection, detect } = await renderMap();

    const control = element.querySelector<HTMLButtonElement>('.dependency-map__access button');
    control?.click();
    detect();

    expect(selection.selectedComponentId()).not.toBe('');
    expect(control?.getAttribute('aria-pressed')).toBe('true');
  });

  it('explains both kinds of dependency in the legend', async () => {
    const { element } = await renderMap();
    const legend = element.querySelector('.dependency-map__legend');

    expect(legend?.textContent).toContain('Production dependency');
    expect(legend?.textContent).toContain('Test-only dependency');
  });
});
