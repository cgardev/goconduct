import { Component, provideZonelessChangeDetection, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ComponentTypesStore } from '../../../kernel/graph/component-types.store';
import { fakeGraph, fakeTypeDeclaration, provideFakeClients } from '../../../testing/fake-clients';
import { TypeDiagramComponent } from './type-diagram.component';
import { buildTypeDiagramModel, type TypeDiagramModel } from './type-diagram.model';

const COMPONENT = 'internal/module/orders';

const TYPES = [
  fakeTypeDeclaration(`${COMPONENT}.Order`, 'struct', {
    component: COMPONENT,
    implements: [
      { id: 'internal/library/telemetry.Writer', component: 'internal/library/telemetry' },
    ],
  }),
  fakeTypeDeclaration(`${COMPONENT}.Line`, 'struct', { component: COMPONENT }),
];

@Component({
  imports: [TypeDiagramComponent],
  template: '<app-type-diagram [model]="model()" (navigateComponent)="navigated.push($event)" />',
})
class HostComponent {
  readonly model = signal<TypeDiagramModel>(buildTypeDiagramModel(TYPES, [], new Set(), ''));
  readonly navigated: string[] = [];
}

interface Rendered {
  readonly element: HTMLElement;
  readonly host: HostComponent;
  readonly store: ComponentTypesStore;
  readonly detect: () => void;
}

// The diagram itself is JointJS over the SVG geometry API, which jsdom does
// not implement; the component skips it there. This file covers the hidden
// access list, which carries the same selection, collapse, and navigation
// actions for the keyboard.
async function renderDiagram(): Promise<Rendered> {
  TestBed.configureTestingModule({
    providers: [
      provideZonelessChangeDetection(),
      ...(provideFakeClients(fakeGraph(), [], TYPES) as never[]),
    ],
  });

  const harness = TestBed.createComponent(HostComponent);
  harness.detectChanges();
  await new Promise((resolve) => setTimeout(resolve, 0));
  harness.detectChanges();

  return {
    element: harness.nativeElement as HTMLElement,
    host: harness.componentInstance,
    store: TestBed.inject(ComponentTypesStore),
    detect: () => harness.detectChanges(),
  };
}

describe('TypeDiagramComponent', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('offers accessible selection and collapse controls per diagrammed type', async () => {
    const { element } = await renderDiagram();

    const controls = element.querySelectorAll('.type-diagram__access button');
    // Two types with two controls each, plus one external navigation control.
    expect(controls).toHaveLength(5);
    expect(controls[0]?.textContent).toContain('Inspect');
    expect(controls[1]?.textContent).toContain('Collapse');
  });

  it('selects the type behind a control', async () => {
    const { element, store, detect } = await renderDiagram();

    const control = element.querySelector<HTMLButtonElement>('.type-diagram__access button');
    control?.click();
    detect();

    expect(store.selectedTypeId()).toBe(`${COMPONENT}.Line`);
    expect(control?.getAttribute('aria-pressed')).toBe('true');
  });

  it('collapses and expands a type from the keyboard', async () => {
    const { element, store, detect } = await renderDiagram();

    const controls = element.querySelectorAll<HTMLButtonElement>('.type-diagram__access button');
    controls[1]?.click();
    detect();
    expect(store.collapsedIds().has(`${COMPONENT}.Line`)).toBe(true);
    expect(controls[1]?.textContent).toContain('Expand');

    controls[1]?.click();
    detect();
    expect(store.collapsedIds().has(`${COMPONENT}.Line`)).toBe(false);
  });

  it('navigates to the component behind an external type', async () => {
    const { element, host, detect } = await renderDiagram();

    const controls = element.querySelectorAll<HTMLButtonElement>('.type-diagram__access button');
    controls[controls.length - 1]?.click();
    detect();

    expect(host.navigated).toEqual(['internal/library/telemetry']);
  });

  it('explains every node kind and relation kind in the legend', async () => {
    const { element } = await renderDiagram();
    const legend = element.querySelector('.type-diagram__legend');

    for (const label of [
      'Struct',
      'Interface',
      'Alias',
      'Basic',
      'Implements',
      'Embeds',
      'References',
    ]) {
      expect(legend?.textContent).toContain(label);
    }
  });
});
