import { DecimalPipe } from '@angular/common';
import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { TuiButton, TuiIcon } from '@taiga-ui/core';
import { ComponentSelectionStore } from '../../../kernel/graph/component-selection.store';

/**
 * Drawer with the coupling metrics and the direct dependencies of the selected
 * component.
 *
 * It reads the selection from the store rather than from an input, so the map
 * and the components table open the same drawer without either page knowing
 * about the other.
 */
@Component({
  selector: 'app-component-detail',
  imports: [DecimalPipe, TuiButton, TuiIcon],
  templateUrl: './component-detail.component.html',
  styleUrl: './component-detail.component.less',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ComponentDetailComponent {
  protected readonly selection = inject(ComponentSelectionStore);
}
