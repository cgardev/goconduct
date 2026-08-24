import { Component, effect, ElementRef, inject, output, viewChild } from '@angular/core';
import { TuiButton } from '@taiga-ui/core';
import type { IncomingTypeRelation, TypeReference } from 'lib-api-gen/gen/v1/graph_pb';
import { ComponentTypesStore } from '../../../kernel/graph/component-types.store';
import { typeShortName } from '../type-diagram/type-diagram.model';

/**
 * Panel that reads the selected Go type: its identity, fields, methods, and
 * relations.
 *
 * It reads the selection from the store rather than from an input, so the
 * diagram and its accessible list open the same panel. A relation inside the
 * component selects that type in place; a relation of another component
 * carries that component's identifier, and following it asks the page to
 * navigate there.
 *
 * The panel is not a dialog: the diagram behind it stays readable and
 * operable, which is the point of selecting one type after another. It
 * therefore traps no focus and darkens nothing. It does close on `Escape`,
 * because a reader who opened it with the keyboard needs a way back that is
 * not a hunt for the close button.
 */
@Component({
  selector: 'app-type-detail',
  imports: [TuiButton],
  templateUrl: './type-detail.component.html',
  styleUrl: './type-detail.component.less',
  host: {
    '(document:keydown.escape)': 'close()',
  },
})
export class TypeDetailComponent {
  protected readonly store = inject(ComponentTypesStore);

  /** The reader follows a relation into another component. */
  readonly navigateComponent = output<string>();

  private readonly panel = viewChild<ElementRef<HTMLElement>>('panel');

  constructor() {
    // Moving focus into the panel is what makes the keyboard path work: the
    // next Tab continues inside the details instead of resuming behind them.
    effect(() => {
      if (this.store.selectedTypeId() === '') {
        return;
      }
      this.panel()?.nativeElement.focus();
    });
  }

  protected close(): void {
    this.store.closeDetail();
  }

  /** Selects a local relation target, or navigates to the other component. */
  protected follow(reference: TypeReference): void {
    if (reference.component === this.store.componentId()) {
      this.store.selectType(reference.id);
      return;
    }
    this.navigateComponent.emit(reference.component);
  }

  protected shortName(identifier: string): string {
    return typeShortName(identifier);
  }

  /** The incoming relations of the selected type, grouped by what each source does. */
  protected incomingGroups(): readonly {
    kind: string;
    title: string;
    relations: readonly IncomingTypeRelation[];
  }[] {
    const relations = this.store.selectedIncoming();
    return [
      { kind: 'implements', title: 'Implemented by' },
      { kind: 'embeds', title: 'Embedded by' },
      { kind: 'references', title: 'Referenced by' },
    ].map((group) => ({
      ...group,
      relations: relations.filter((relation) => relation.kind === group.kind),
    }));
  }

  protected external(reference: TypeReference): boolean {
    return reference.component !== this.store.componentId();
  }
}
