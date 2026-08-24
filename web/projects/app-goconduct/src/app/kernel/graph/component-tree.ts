/** One node of the component tree: a path segment, a component, or both. */
export interface ComponentTreeNode {
  /** Rendered label: one segment, or a compacted chain like `internal/module`. */
  readonly label: string;
  /** Full path from the repository root to this node. */
  readonly path: string;
  /** Identifier of the component the path names, empty for a plain directory. */
  readonly componentId: string;
  readonly children: readonly ComponentTreeNode[];
}

interface MutableNode {
  label: string;
  path: string;
  componentId: string;
  children: Map<string, MutableNode>;
}

/**
 * Builds the directory tree of the analyzed components.
 *
 * The tree mirrors the repository layout, so a reader finds a component the
 * way they find its files. A chain of directories with a single child
 * compacts into one node, the way code editors compact folders, so the tree
 * stays shallow.
 */
export function buildComponentTree(identifiers: readonly string[]): readonly ComponentTreeNode[] {
  const roots = new Map<string, MutableNode>();
  for (const identifier of [...identifiers].sort((first, second) => first.localeCompare(second))) {
    let level = roots;
    let path = '';
    let node: MutableNode | undefined;
    for (const segment of identifier.split('/')) {
      path = path === '' ? segment : `${path}/${segment}`;
      node = level.get(segment);
      if (node === undefined) {
        node = { label: segment, path, componentId: '', children: new Map() };
        level.set(segment, node);
      }
      level = node.children;
    }
    if (node !== undefined) {
      node.componentId = identifier;
    }
  }
  return [...roots.values()].map((root) => finalize(compact(root))).sort(byBranchThenLabel);
}

/**
 * Returns the paths of every ancestor of one component, so a deep link can
 * open the tree down to its selection. An unknown component returns no paths.
 */
export function ancestorPaths(
  roots: readonly ComponentTreeNode[],
  componentId: string,
): readonly string[] {
  return findAncestors(roots, componentId) ?? [];
}

function findAncestors(
  nodes: readonly ComponentTreeNode[],
  componentId: string,
): readonly string[] | undefined {
  for (const node of nodes) {
    if (node.componentId === componentId) {
      return [];
    }
    const below = findAncestors(node.children, componentId);
    if (below !== undefined) {
      return [node.path, ...below];
    }
  }
  return undefined;
}

// compact merges a directory that only relays to one child into that child,
// so `internal` and `module` render as one `internal/module` node.
function compact(node: MutableNode): MutableNode {
  let current = node;
  while (current.componentId === '' && current.children.size === 1) {
    const [child] = current.children.values();
    if (child === undefined) {
      break;
    }
    current = { ...child, label: `${current.label}/${child.label}` };
  }
  return current;
}

// finalize orders every level the way a file tree does: the nodes that can
// open first, then the leaves, each group alphabetically.
function finalize(node: MutableNode): ComponentTreeNode {
  const children = [...node.children.values()]
    .map((child) => finalize(compact(child)))
    .sort(byBranchThenLabel);
  return {
    label: node.label,
    path: node.path,
    componentId: node.componentId,
    children,
  };
}

function byBranchThenLabel(first: ComponentTreeNode, second: ComponentTreeNode): number {
  const firstIsBranch = first.children.length > 0 ? 0 : 1;
  const secondIsBranch = second.children.length > 0 ? 0 : 1;
  return firstIsBranch - secondIsBranch || first.label.localeCompare(second.label);
}
