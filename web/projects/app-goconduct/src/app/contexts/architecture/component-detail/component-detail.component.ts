import { DecimalPipe } from '@angular/common';
import {
  ChangeDetectionStrategy,
  Component,
  effect,
  ElementRef,
  inject,
  viewChild,
} from '@angular/core';
import { TuiButton, TuiIcon } from '@taiga-ui/core';
import { ComponentSelectionStore } from '../../../kernel/graph/component-selection.store';

/**
 * Panel with the coupling metrics and the direct dependencies of the selected
 * component.
 *
 * It reads the selection from the store rather than from an input, so the map
 * and the components table open the same panel without either page knowing
 * about the other.
 *
 * The panel is not a dialog: the collection behind it stays readable and
 * operable, which is the point of selecting one row after another. It therefore
 * traps no focus and darkens nothing. It does close on `Escape`, because a
 * reader who opened it with the keyboard needs a way back that is not a hunt
 * for the close button.
 */
@Component({
  selector: 'app-component-detail',
  imports: [DecimalPipe, TuiButton, TuiIcon],
  templateUrl: './component-detail.component.html',
  styleUrl: './component-detail.component.less',
  host: {
    '(document:keydown.escape)': 'close()',
  },
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ComponentDetailComponent {
  protected readonly selection = inject(ComponentSelectionStore);

  private readonly panel = viewChild<ElementRef<HTMLElement>>('panel');

  constructor() {
    // Moving focus into the panel is what makes the keyboard path work: the
    // next Tab continues inside the details instead of resuming in the table
    // behind them.
    effect(() => {
      if (this.selection.selectedComponentId() === '') {
        return;
      }
      this.panel()?.nativeElement.focus();
    });
  }

  protected close(): void {
    this.selection.close();
  }
}
