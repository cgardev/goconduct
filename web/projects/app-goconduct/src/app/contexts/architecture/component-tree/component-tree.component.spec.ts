import { Component, provideZonelessChangeDetection, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ComponentTreeComponent } from './component-tree.component';

const COMPONENTS = [
  'cmd/goconduct',
  'internal/library/telemetry',
  'internal/module/orders',
] as const;

@Component({
  imports: [ComponentTreeComponent],
  template: `
    <app-component-tree
      [components]="components"
      [selectedId]="selectedId()"
      (select)="selectedId.set($event)"
    />
  `,
})
class HostComponent {
  readonly components = COMPONENTS;
  readonly selectedId = signal('');
}

interface Rendered {
  readonly element: HTMLElement;
  readonly host: HostComponent;
  readonly detect: () => void;
}

function renderTree(selectedId = ''): Rendered {
  TestBed.configureTestingModule({
    providers: [provideZonelessChangeDetection()],
  });
  const harness = TestBed.createComponent(HostComponent);
  harness.componentInstance.selectedId.set(selectedId);
  harness.detectChanges();
  return {
    element: harness.nativeElement as HTMLElement,
    host: harness.componentInstance,
    detect: () => harness.detectChanges(),
  };
}

describe('ComponentTreeComponent', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('offers one control per component, under its directory', () => {
    const { element } = renderTree();

    const labels = [...element.querySelectorAll('.component-tree__item--component')].map(
      (control) => control.textContent?.trim(),
    );
    expect(labels).toContain('cmd/goconduct');
  });

  it('selects the component behind a control', () => {
    const { element, host, detect } = renderTree();

    const control = [...element.querySelectorAll<HTMLButtonElement>('button')].find(
      (candidate) => candidate.textContent?.trim() === 'cmd/goconduct',
    );
    control?.click();
    detect();

    expect(host.selectedId()).toBe('cmd/goconduct');
    expect(control?.getAttribute('aria-pressed')).toBe('true');
  });

  /** A deep link arrives with the tree closed; the selection opens its ancestors. */
  it('reveals the selected component of a deep link', () => {
    const { element } = renderTree('internal/module/orders');

    const selected = element.querySelector('.component-tree__item--selected');
    expect(selected?.textContent?.trim()).toBe('module/orders');
  });
});
