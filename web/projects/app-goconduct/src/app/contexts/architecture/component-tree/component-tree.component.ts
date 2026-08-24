import { Component, computed, effect, input, output, signal } from '@angular/core';
import type { TuiHandler } from '@taiga-ui/cdk';
import { TuiTree } from '@taiga-ui/kit';
import {
  ancestorPaths,
  buildComponentTree,
  type ComponentTreeNode,
} from '../../../kernel/graph/component-tree';

/**
 * The analyzed components as a file-style tree.
 *
 * The tree mirrors the repository layout, so a reader finds a component the
 * way they find its files and sees at a glance what nests inside what. A
 * node that names a component selects it; a plain directory only opens and
 * closes. The ancestors of the selection open on their own, so a deep link
 * reveals its component.
 */
@Component({
  selector: 'app-component-tree',
  imports: [TuiTree],
  templateUrl: './component-tree.component.html',
  styleUrl: './component-tree.component.less',
})
export class ComponentTreeComponent {
  /** Identifiers of every analyzed component. */
  readonly components = input.required<readonly string[]>();

  /** Identifier of the selected component, empty when nothing is selected. */
  readonly selectedId = input.required<string>();

  /** The reader picks one component. */
  readonly select = output<string>();

  protected readonly roots = computed(() => buildComponentTree(this.components()));

  private readonly openPaths = signal<ReadonlySet<string>>(new Set());

  /** Expansion state per node, in the shape `tui-tree` consumes. */
  protected readonly expansion = computed(() => {
    const open = this.openPaths();
    const expansion = new Map<ComponentTreeNode, boolean>();
    const visit = (node: ComponentTreeNode): void => {
      expansion.set(node, open.has(node.path));
      node.children.forEach(visit);
    };
    this.roots().forEach(visit);
    return expansion;
  });

  protected readonly childrenOf: TuiHandler<ComponentTreeNode, readonly ComponentTreeNode[]> = (
    node,
  ) => node.children;

  constructor() {
    // A deep link arrives with the tree closed; opening the ancestors of the
    // selection reveals it without a hunt.
    effect(() => {
      const selected = this.selectedId();
      if (selected === '') {
        return;
      }
      const reveal = ancestorPaths(this.roots(), selected);
      if (reveal.length === 0) {
        return;
      }
      this.openPaths.update((current) => new Set([...current, ...reveal]));
    });
  }

  protected toggle(node: ComponentTreeNode): void {
    this.openPaths.update((current) => {
      const next = new Set(current);
      if (!next.delete(node.path)) {
        next.add(node.path);
      }
      return next;
    });
  }
}
