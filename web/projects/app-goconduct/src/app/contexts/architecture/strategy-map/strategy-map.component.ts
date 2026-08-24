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
import { supportsSvgGeometry } from '../../../kernel/diagram/rendering-support';
import type { StrategyReport } from '../../../kernel/graph/strategy-report';
import { createStrategyMap, type StrategyMap } from './strategy-map.diagram';

/**
 * The de facto layer map: one node per strategic group on its inferred layer.
 *
 * JointJS renders the map, as `LIBS.md` assigns to dependency maps. The
 * surface answers the pointer, while a visually hidden list of buttons gives
 * the keyboard and assistive technology the same selection.
 */
@Component({
  selector: 'app-strategy-map',
  templateUrl: './strategy-map.component.html',
  styleUrl: './strategy-map.component.less',
})
export class StrategyMapComponent {
  private readonly surface = viewChild.required<ElementRef<HTMLElement>>('surface');

  /** The mined strategy to draw. */
  readonly report = input.required<StrategyReport>();

  /** The reader picks one group to read its dependencies. */
  readonly select = output<string>();

  private readonly diagram = signal<StrategyMap | undefined>(undefined);
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
      void createStrategyMap(this.surface().nativeElement, (groupId) =>
        this.select.emit(groupId),
      ).then((diagram) => {
        if (this.destroyed) {
          diagram.dispose();
          return;
        }
        this.diagram.set(diagram);
      });
    });

    effect(() => {
      this.diagram()?.render(this.report());
    });
  }
}
