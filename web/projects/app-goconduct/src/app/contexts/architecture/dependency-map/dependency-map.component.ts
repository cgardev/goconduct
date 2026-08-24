import {
  afterNextRender,
  Component,
  DestroyRef,
  effect,
  ElementRef,
  inject,
  input,
  signal,
  viewChild,
} from '@angular/core';
import { supportsSvgGeometry } from '../../../kernel/diagram/rendering-support';
import { ComponentSelectionStore } from '../../../kernel/graph/component-selection.store';
import type { GraphLayout } from '../../../kernel/graph/graph-layout';
import { createDependencyMapDiagram, type DependencyMapDiagram } from './dependency-map.diagram';

/**
 * The repository map: one directed node per component, one link per
 * dependency.
 *
 * JointJS (`@joint/core`) renders the diagram, as `LIBS.md` assigns to
 * dependency maps. The JointJS surface answers the pointer, while a visually
 * hidden list of buttons gives the keyboard and assistive technology the same
 * selection.
 *
 * The map reads the selection from the store, so selecting a node opens the
 * same detail drawer the chart and the table open.
 */
@Component({
  selector: 'app-dependency-map',
  templateUrl: './dependency-map.component.html',
  styleUrl: './dependency-map.component.less',
})
export class DependencyMapComponent {
  protected readonly selection = inject(ComponentSelectionStore);
  private readonly surface = viewChild.required<ElementRef<HTMLElement>>('surface');

  /** The positioned nodes and links to draw. */
  readonly layout = input.required<GraphLayout>();

  // The engine loads on demand, so the diagram exists only after the first
  // render. The signal lets the effect below wait for it.
  private readonly diagram = signal<DependencyMapDiagram | undefined>(undefined);
  private destroyed = false;

  constructor() {
    inject(DestroyRef).onDestroy(() => {
      this.destroyed = true;
      this.diagram()?.dispose();
    });

    afterNextRender(() => {
      if (!supportsSvgGeometry()) {
        return;
      }
      void createDependencyMapDiagram(this.surface().nativeElement, (id) =>
        this.selection.select(id),
      ).then((diagram) => {
        if (this.destroyed) {
          diagram.dispose();
          return;
        }
        this.diagram.set(diagram);
      });
    });

    effect(() => {
      const diagram = this.diagram();
      if (diagram === undefined) {
        return;
      }
      diagram.render(this.layout(), this.selection.selectedComponentId());
    });
  }
}
