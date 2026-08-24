import {
  afterNextRender,
  Component,
  DestroyRef,
  effect,
  ElementRef,
  inject,
  input,
  output,
  signal,
  viewChild,
} from '@angular/core';
import { TuiButton } from '@taiga-ui/core';
import { supportsSvgGeometry } from '../../../kernel/diagram/rendering-support';
import { ComponentTypesStore } from '../../../kernel/graph/component-types.store';
import { createTypeDiagram, type TypeDiagram } from './type-diagram.diagram';
import type { TypeDiagramModel } from './type-diagram.model';

/**
 * The UML-style diagram of one component's Go types.
 *
 * JointJS (`@joint/core`) renders the diagram, as `LIBS.md` assigns to type
 * diagrams: one custom node per type with a header, one row per field or
 * method with its own port, and one link style per relation kind. The
 * JointJS surface answers the pointer, while a visually hidden list of
 * buttons gives the keyboard and assistive technology the same selection,
 * collapse, and navigation actions. The zoom buttons are the keyboard
 * equivalent of the wheel zoom and the drag pan, which stay inside the
 * canvas.
 */
@Component({
  selector: 'app-type-diagram',
  imports: [TuiButton],
  templateUrl: './type-diagram.component.html',
  styleUrl: './type-diagram.component.less',
})
export class TypeDiagramComponent {
  protected readonly store = inject(ComponentTypesStore);
  private readonly surface = viewChild.required<ElementRef<HTMLElement>>('surface');

  /** The positioned nodes and links to draw. */
  readonly model = input.required<TypeDiagramModel>();

  /** The reader wants the types of another component. */
  readonly navigateComponent = output<string>();

  // The engine loads on demand, so the diagram exists only after the first
  // render. The signal lets the effect below wait for it.
  private readonly diagram = signal<TypeDiagram | undefined>(undefined);
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
      void createTypeDiagram(this.surface().nativeElement, {
        onSelect: (typeId) => this.store.selectType(typeId),
        onToggle: (typeId) => this.store.toggleCollapsed(typeId),
        onNavigate: (componentId) => this.navigateComponent.emit(componentId),
      }).then((diagram) => {
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
      diagram.render(this.model(), this.store.selectedTypeId());
    });
  }

  protected zoomIn(): void {
    this.diagram()?.zoom(1.2);
  }

  protected zoomOut(): void {
    this.diagram()?.zoom(1 / 1.2);
  }

  protected resetView(): void {
    this.diagram()?.resetView();
  }

  protected collapseLabel(id: string, name: string): string {
    return this.store.collapsedIds().has(id) ? `Expand ${name}` : `Collapse ${name}`;
  }
}
