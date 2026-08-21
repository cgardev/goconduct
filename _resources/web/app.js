"use strict";

const svgNamespace = "http://www.w3.org/2000/svg";
const numberFormatter = new Intl.NumberFormat("en-GB");

const kindDefinitions = [
  { identifier: "application", label: "Application", plural: "Applications" },
  {
    identifier: "application-module",
    label: "Application module",
    plural: "Application modules",
  },
  { identifier: "shared-module", label: "Shared module", plural: "Shared modules" },
  { identifier: "library", label: "Library", plural: "Libraries" },
  { identifier: "infrastructure", label: "Infrastructure", plural: "Infrastructure" },
  { identifier: "development", label: "Development tool", plural: "Development tools" },
];

const concernLabels = {
  "production-depends-on-development": "Production code imports a development tool.",
  "library-depends-on-feature": "A library imports a feature.",
  "shared-foundation-depends-on-application": "Shared foundation code imports application code.",
  "cross-application-module-dependency":
    "An application module imports a module from a different application.",
  "stable-dependency-principle":
    "SDP violation: this component depends on a less stable component.",
};

const impactMetricDefinitions = Object.freeze({
  consumers: {
    field: "afferentCoupling",
    label: "afferent coupling (Cₐ)",
    shortLabel: "Cₐ",
  },
  efferent: {
    field: "efferentCoupling",
    label: "efferent coupling (Cₑ)",
    shortLabel: "Cₑ",
  },
  packages: {
    field: "productionImporterPackages",
    label: "production importer packages",
    shortLabel: "packages",
  },
  transitive: {
    field: "transitiveDependants",
    label: "indirect production consumers",
    shortLabel: "indirect",
  },
  files: {
    field: "sourceFiles",
    label: "Go source files",
    shortLabel: "Go files",
  },
});

const layoutOptions = Object.freeze({
  nodeWidth: 196,
  nodeHeight: 46,
  verticalStep: 62,
  columnStep: 222,
  kindGap: 58,
  maximumRows: 14,
  left: 54,
  top: 86,
  right: 54,
  bottom: 64,
  fitPadding: 28,
  minimumScale: 0.12,
  maximumScale: 2.4,
});

const impactLayoutOptions = Object.freeze({
  padding: 8,
  gap: 3,
  zeroWeight: 0.2,
  minimumWidth: 640,
  minimumHeight: 480,
});

const elements = {
  connectionStatus: document.getElementById("connectionStatus"),
  connectionLabel: document.getElementById("connectionLabel"),
  revisionLabel: document.getElementById("revisionLabel"),
  modulePath: document.getElementById("modulePath"),
  summaryComponents: document.getElementById("summaryComponents"),
  summaryKinds: document.getElementById("summaryKinds"),
  summaryRelationships: document.getElementById("summaryRelationships"),
  summaryTests: document.getElementById("summaryTests"),
  summaryReach: document.getElementById("summaryReach"),
  summaryReachNote: document.getElementById("summaryReachNote"),
  summaryConcerns: document.getElementById("summaryConcerns"),
  summaryCycles: document.getElementById("summaryCycles"),
  searchInput: document.getElementById("searchInput"),
  kindFilters: document.getElementById("kindFilters"),
  testToggle: document.getElementById("testToggle"),
  graphEyebrow: document.getElementById("graphEyebrow"),
  graphTitle: document.getElementById("graphTitle"),
  graphDescription: document.getElementById("graphDescription"),
  graphViewport: document.getElementById("graphViewport"),
  impactMap: document.getElementById("impactMap"),
  dependencyMap: document.getElementById("dependencyMap"),
  graphEmpty: document.getElementById("graphEmpty"),
  legend: document.getElementById("legend"),
  impactView: document.getElementById("impactView"),
  dependencyView: document.getElementById("dependencyView"),
  impactMetricControl: document.getElementById("impactMetricControl"),
  impactMetric: document.getElementById("impactMetric"),
  dependencyControls: document.getElementById("dependencyControls"),
  impactLegend: document.getElementById("impactLegend"),
  impactLegendText: document.getElementById("impactLegendText"),
  edgeLegend: document.getElementById("edgeLegend"),
  zoomOut: document.getElementById("zoomOut"),
  fitGraph: document.getElementById("fitGraph"),
  zoomIn: document.getElementById("zoomIn"),
  detailPanel: document.getElementById("detailPanel"),
  usageRanking: document.getElementById("usageRanking"),
  unusedRanking: document.getElementById("unusedRanking"),
  zoneOfPainRanking: document.getElementById("zoneOfPainRanking"),
  diagnostics: document.getElementById("diagnostics"),
  diagnosticsSummary: document.getElementById("diagnosticsSummary"),
  diagnosticsList: document.getElementById("diagnosticsList"),
};

const state = {
  graph: null,
  selectedIdentifier: "",
  visibleKinds: new Set(kindDefinitions.map((definition) => definition.identifier)),
  includeTests: true,
  search: "",
  viewMode: "impact",
  impactMetric: "consumers",
  contentGroup: null,
  contentBounds: { width: 0, height: 0 },
  transform: { x: 0, y: 0, scale: 1 },
  dragging: null,
  graphRequestRunning: false,
  graphRequestPending: false,
  graphRequestPromise: null,
  eventStreamConnected: false,
};

function createElement(tagName, className, text) {
  const element = document.createElement(tagName);
  if (className) {
    element.className = className;
  }
  if (text !== undefined) {
    element.textContent = text;
  }
  return element;
}

function createSvgElement(tagName, className) {
  const element = document.createElementNS(svgNamespace, tagName);
  if (className) {
    element.setAttribute("class", className);
  }
  return element;
}

function kindDefinition(identifier) {
  return kindDefinitions.find((definition) => definition.identifier === identifier);
}

function compareIdentifiers(first, second) {
  if (first.id < second.id) {
    return -1;
  }
  if (first.id > second.id) {
    return 1;
  }
  return 0;
}

function compareUsage(first, second) {
  return (
    second.productionDependants - first.productionDependants ||
    second.productionImporterPackages - first.productionImporterPackages ||
    second.applicationReach - first.applicationReach ||
    compareIdentifiers(first, second)
  );
}

function formatRatio(value) {
  const ratio = Number(value);
  return Number.isFinite(ratio) ? ratio.toFixed(2) : "0.00";
}

function normalizeSearch(value) {
  return value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLocaleLowerCase("en-GB")
    .trim();
}

function componentMatchesSearch(component) {
  if (!state.search) {
    return true;
  }
  const searchable = [
    component.id,
    component.name,
    component.application,
    ...(component.applications || []),
  ].join(" ");
  return normalizeSearch(searchable).includes(state.search);
}

function graphComponents() {
  if (!state.graph) {
    return [];
  }
  return state.graph.components.filter(
    (component) => state.visibleKinds.has(component.kind) && componentMatchesSearch(component),
  );
}

function graphRelationships(visibleIdentifiers) {
  if (!state.graph) {
    return [];
  }
  return state.graph.relationships.filter(
    (relationship) =>
      visibleIdentifiers.has(relationship.source) &&
      visibleIdentifiers.has(relationship.target) &&
      (state.includeTests || !relationship.testOnly),
  );
}

function impactMetricDefinition() {
  return impactMetricDefinitions[state.impactMetric] || impactMetricDefinitions.consumers;
}

function impactValue(component) {
  const value = Number(component[impactMetricDefinition().field]);
  return Number.isFinite(value) && value > 0 ? value : 0;
}

function compareImpact(first, second) {
  return impactValue(second) - impactValue(first) || compareUsage(first, second);
}

function totalTreemapWeight(items) {
  return items.reduce((total, item) => total + item.weight, 0);
}

function treemapSplitIndex(items, totalWeight) {
  const midpoint = totalWeight / 2;
  let firstWeight = items[0].weight;
  let index = 1;
  while (index < items.length - 1) {
    const nextWeight = firstWeight + items[index].weight;
    if (Math.abs(midpoint - nextWeight) > Math.abs(midpoint - firstWeight)) {
      break;
    }
    firstWeight = nextWeight;
    index += 1;
  }
  return index;
}

function layoutTreemap(items, bounds, tiles = []) {
  if (items.length === 0) {
    return tiles;
  }
  if (items.length === 1) {
    tiles.push({ ...bounds, ...items[0] });
    return tiles;
  }

  const totalWeight = totalTreemapWeight(items);
  const splitIndex = treemapSplitIndex(items, totalWeight);
  const firstItems = items.slice(0, splitIndex);
  const secondItems = items.slice(splitIndex);
  const firstRatio = totalTreemapWeight(firstItems) / totalWeight;

  if (bounds.width >= bounds.height) {
    const firstWidth = bounds.width * firstRatio;
    layoutTreemap(firstItems, { ...bounds, width: firstWidth }, tiles);
    layoutTreemap(
      secondItems,
      {
        x: bounds.x + firstWidth,
        y: bounds.y,
        width: bounds.width - firstWidth,
        height: bounds.height,
      },
      tiles,
    );
    return tiles;
  }

  const firstHeight = bounds.height * firstRatio;
  layoutTreemap(firstItems, { ...bounds, height: firstHeight }, tiles);
  layoutTreemap(
    secondItems,
    {
      x: bounds.x,
      y: bounds.y + firstHeight,
      width: bounds.width,
      height: bounds.height - firstHeight,
    },
    tiles,
  );
  return tiles;
}

function shortenedImpactName(name, width) {
  const maximumLength = Math.max(5, Math.floor((width - 20) / 7));
  if (name.length <= maximumLength) {
    return name;
  }
  return `${name.slice(0, Math.max(1, maximumLength - 1))}…`;
}

function appendImpactTile(container, tile, rank, concernedIdentifiers) {
  const { component, value, width, height } = tile;
  const inset = Math.min(
    impactLayoutOptions.gap / 2,
    Math.max(0, width / 5),
    Math.max(0, height / 5),
  );
  const contentWidth = Math.max(0.5, width - inset * 2);
  const contentHeight = Math.max(0.5, height - inset * 2);
  const rankVisible = contentWidth >= 145 && contentHeight >= 32;
  const classes = ["impact-map__tile"];
  if (component.id === state.selectedIdentifier) {
    classes.push("impact-map__tile--selected");
  }
  if (value === 0) {
    classes.push("impact-map__tile--zero");
  }
  if (component.inCycle) {
    classes.push("impact-map__tile--cycle");
  }
  if (component.inZoneOfPain) {
    classes.push("impact-map__tile--pain");
  }

  const node = createSvgElement("g", classes.join(" "));
  node.dataset.kind = component.kind;
  node.setAttribute("transform", `translate(${tile.x} ${tile.y})`);
  node.setAttribute("tabindex", "0");
  node.setAttribute("role", "button");
  node.setAttribute(
    "aria-label",
    `${component.name}, ${numberFormatter.format(value)} ${impactMetricDefinition().label}`,
  );

  const box = createSvgElement("rect", "impact-map__tile-box");
  box.setAttribute("x", String(inset));
  box.setAttribute("y", String(inset));
  box.setAttribute("width", String(contentWidth));
  box.setAttribute("height", String(contentHeight));
  box.setAttribute("rx", String(Math.min(6, contentWidth / 5, contentHeight / 5)));
  node.append(box);

  const title = createSvgElement("title");
  title.textContent =
    `${component.id}\nCₐ ${component.afferentCoupling} · Cₑ ${component.efferentCoupling}\n` +
    `Instability I ${formatRatio(component.instability)} · ` +
    `Abstractness A ${formatRatio(component.abstractness)}\n` +
    `Main-sequence distance D ${formatRatio(component.mainSequenceDistance)}\n` +
    `${component.abstractTypes} abstract types · ${component.concreteTypes} concrete types`;
  node.append(title);

  if (contentWidth >= 72 && contentHeight >= 32) {
    const name = createSvgElement("text", "impact-map__tile-name");
    name.setAttribute("x", String(inset + 10));
    name.setAttribute("y", String(inset + 18));
    name.textContent = shortenedImpactName(component.name, contentWidth - (rankVisible ? 42 : 0));
    node.append(name);
  }
  if (rankVisible) {
    const rankLabel = createSvgElement("text", "impact-map__tile-rank");
    rankLabel.setAttribute("x", String(width - inset - 10));
    rankLabel.setAttribute("y", String(inset + 17));
    rankLabel.setAttribute("text-anchor", "end");
    rankLabel.textContent = `#${String(rank).padStart(2, "0")}`;
    node.append(rankLabel);
  }
  if (contentWidth >= 88 && contentHeight >= 52) {
    const metric = createSvgElement("text", "impact-map__tile-value");
    metric.setAttribute("x", String(inset + 10));
    metric.setAttribute("y", String(inset + 38));
    metric.textContent =
      `${numberFormatter.format(value)} ${impactMetricDefinition().shortLabel}`;
    node.append(metric);
  }
  if (contentWidth >= 122 && contentHeight >= 76) {
    const kind = createSvgElement("text", "impact-map__tile-kind");
    kind.setAttribute("x", String(inset + 10));
    kind.setAttribute("y", String(height - inset - 10));
    kind.textContent = kindDefinition(component.kind).label;
    node.append(kind);
  }
  if (
    (concernedIdentifiers.has(component.id) || component.inZoneOfPain) &&
    contentWidth >= 24 &&
    contentHeight >= 24
  ) {
    const alert = createSvgElement("circle", "impact-map__tile-alert");
    alert.setAttribute("cx", String(width - inset - 9));
    alert.setAttribute("cy", String(height - inset - 9));
    alert.setAttribute("r", "3.5");
    node.append(alert);
  }

  node.addEventListener("click", (event) => {
    event.stopPropagation();
    selectComponent(component.id, false);
  });
  node.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      selectComponent(component.id, false);
    }
  });
  container.append(node);
}

function renderImpactMap() {
  if (!state.graph) {
    return;
  }
  const components = graphComponents().sort(compareImpact);
  elements.graphEmpty.hidden = components.length !== 0;
  elements.impactMap.replaceChildren();

  const width = Math.max(elements.graphViewport.clientWidth, impactLayoutOptions.minimumWidth);
  const height = Math.max(elements.graphViewport.clientHeight, impactLayoutOptions.minimumHeight);
  elements.impactMap.setAttribute("viewBox", `0 0 ${width} ${height}`);

  const items = components.map((component) => {
    const value = impactValue(component);
    return {
      component,
      value,
      weight: value > 0 ? value : impactLayoutOptions.zeroWeight,
    };
  });
  const padding = impactLayoutOptions.padding;
  const tiles = layoutTreemap(items, {
    x: padding,
    y: padding,
    width: width - padding * 2,
    height: height - padding * 2,
  });
  const concernedIdentifiers = new Set();
  for (const relationship of state.graph.relationships) {
    if (relationship.concerns.length > 0) {
      concernedIdentifiers.add(relationship.source);
      concernedIdentifiers.add(relationship.target);
    }
  }
  tiles.forEach((tile, index) => {
    appendImpactTile(elements.impactMap, tile, index + 1, concernedIdentifiers);
  });
}

function renderKindControls() {
  elements.kindFilters.replaceChildren();
  elements.legend.replaceChildren();
  const counts = new Map(kindDefinitions.map((definition) => [definition.identifier, 0]));
  for (const component of state.graph?.components || []) {
    counts.set(component.kind, (counts.get(component.kind) || 0) + 1);
  }

  for (const definition of kindDefinitions) {
    const button = createElement("button", "toolbar__kind");
    button.type = "button";
    button.dataset.kind = definition.identifier;
    button.setAttribute("aria-pressed", String(state.visibleKinds.has(definition.identifier)));
    button.setAttribute("aria-label", `Show or hide ${definition.plural.toLocaleLowerCase("en-GB")}`);

    const dot = createElement("span", "toolbar__kind-dot");
    dot.setAttribute("aria-hidden", "true");
    const label = createElement("span", "toolbar__kind-label", definition.label);
    const count = createElement(
      "span",
      "toolbar__kind-count",
      numberFormatter.format(counts.get(definition.identifier) || 0),
    );
    button.append(dot, label, count);
    button.addEventListener("click", () => {
      if (state.visibleKinds.has(definition.identifier)) {
        state.visibleKinds.delete(definition.identifier);
      } else {
        state.visibleKinds.add(definition.identifier);
      }
      button.setAttribute("aria-pressed", String(state.visibleKinds.has(definition.identifier)));
      renderActiveMap(true);
    });
    elements.kindFilters.append(button);

    const legendItem = createElement("span", "legend__item");
    legendItem.dataset.kind = definition.identifier;
    const legendDot = createElement("span", "legend__dot");
    legendDot.setAttribute("aria-hidden", "true");
    legendItem.append(legendDot, document.createTextNode(definition.label));
    elements.legend.append(legendItem);
  }
}

function renderSummary() {
  const { summary, components } = state.graph;
  elements.modulePath.textContent = state.graph.modulePath;
  elements.summaryComponents.textContent = numberFormatter.format(summary.components);
  elements.summaryKinds.textContent =
    `${summary.applications} applications · ${summary.sharedModules + summary.applicationModules} modules · ` +
    `${summary.libraries} libraries`;
  elements.summaryRelationships.textContent = numberFormatter.format(summary.productionRelationships);
  elements.summaryTests.textContent = `${summary.testOnlyRelationships} test-only relationships`;

  const candidates = components
    .filter((component) => !["application", "development"].includes(component.kind))
    .sort(
      (first, second) =>
        second.applicationReach - first.applicationReach ||
        second.transitiveDependants - first.transitiveDependants ||
        compareUsage(first, second),
    );
  const greatestReach = candidates[0];
  elements.summaryReach.textContent = greatestReach?.name || "—";
  elements.summaryReach.title = greatestReach?.id || "";
  elements.summaryReachNote.textContent = greatestReach
    ? `${greatestReach.applicationReach} applications · ${greatestReach.transitiveDependants} indirect consumers`
    : "No shared component";

  elements.summaryConcerns.textContent = numberFormatter.format(
    summary.concerns + summary.cycles + summary.zonesOfPain,
  );
  elements.summaryCycles.textContent =
    `${summary.stableDependencyViolations} SDP violations · ` +
    `${summary.zonesOfPain} pain-zone components · ${summary.cycles} cycles`;
  elements.revisionLabel.textContent = `revision ${state.graph.revision.slice(0, 12)}`;
}

function layoutComponents(components) {
  const positions = new Map();
  const groups = [];
  let currentX = layoutOptions.left;
  let maximumRows = 0;

  for (const definition of kindDefinitions) {
    const members = components
      .filter((component) => component.kind === definition.identifier)
      .sort(compareIdentifiers);
    if (members.length === 0) {
      continue;
    }
    const columnCount = Math.ceil(members.length / layoutOptions.maximumRows);
    const rowCount = Math.min(members.length, layoutOptions.maximumRows);
    groups.push({ definition, x: currentX, columnCount, count: members.length });
    maximumRows = Math.max(maximumRows, rowCount);

    members.forEach((component, index) => {
      const column = Math.floor(index / layoutOptions.maximumRows);
      const row = index % layoutOptions.maximumRows;
      positions.set(component.id, {
        x: currentX + column * layoutOptions.columnStep,
        y: layoutOptions.top + row * layoutOptions.verticalStep,
      });
    });
    currentX += columnCount * layoutOptions.columnStep + layoutOptions.kindGap;
  }

  return {
    positions,
    groups,
    width: Math.max(
      currentX - layoutOptions.kindGap + layoutOptions.right,
      layoutOptions.nodeWidth + layoutOptions.left + layoutOptions.right,
    ),
    height: Math.max(
      layoutOptions.top + maximumRows * layoutOptions.verticalStep + layoutOptions.bottom,
      320,
    ),
  };
}

function appendArrowMarker(definitions) {
  const marker = createSvgElement("marker");
  marker.id = "dependencyArrow";
  marker.setAttribute("viewBox", "0 0 10 10");
  marker.setAttribute("refX", "9");
  marker.setAttribute("refY", "5");
  marker.setAttribute("markerWidth", "5");
  marker.setAttribute("markerHeight", "5");
  marker.setAttribute("orient", "auto-start-reverse");
  const arrow = createSvgElement("path");
  arrow.setAttribute("d", "M 0 0 L 10 5 L 0 10 z");
  arrow.setAttribute("fill", "context-stroke");
  marker.append(arrow);
  definitions.append(marker);
}

function appendGroupGuides(content, layout) {
  for (const group of layout.groups) {
    const label = createSvgElement("text", "dependency-map__column-label");
    label.dataset.kind = group.definition.identifier;
    label.setAttribute("x", String(group.x));
    label.setAttribute("y", "42");
    label.textContent = `${group.definition.plural} · ${group.count}`;
    content.append(label);

    const line = createSvgElement("line", "dependency-map__column-line");
    line.setAttribute("x1", String(group.x));
    line.setAttribute("x2", String(group.x + group.columnCount * layoutOptions.columnStep - 26));
    line.setAttribute("y1", "56");
    line.setAttribute("y2", "56");
    content.append(line);
  }
}

function relationshipPath(source, target) {
  const sourceCenterY = source.y + layoutOptions.nodeHeight / 2;
  const targetCenterY = target.y + layoutOptions.nodeHeight / 2;
  if (source.x < target.x) {
    const startX = source.x + layoutOptions.nodeWidth;
    const endX = target.x;
    const controlX = startX + (endX - startX) / 2;
    return `M ${startX} ${sourceCenterY} C ${controlX} ${sourceCenterY}, ` +
      `${controlX} ${targetCenterY}, ${endX} ${targetCenterY}`;
  }
  if (source.x > target.x) {
    const startX = source.x;
    const endX = target.x + layoutOptions.nodeWidth;
    const controlX = startX - (startX - endX) / 2;
    return `M ${startX} ${sourceCenterY} C ${controlX} ${sourceCenterY}, ` +
      `${controlX} ${targetCenterY}, ${endX} ${targetCenterY}`;
  }
  const edgeX = source.x + layoutOptions.nodeWidth + 32;
  return `M ${source.x + layoutOptions.nodeWidth} ${sourceCenterY} ` +
    `C ${edgeX} ${sourceCenterY}, ${edgeX} ${targetCenterY}, ` +
    `${target.x + layoutOptions.nodeWidth} ${targetCenterY}`;
}

function appendRelationships(content, relationships, positions) {
  for (const relationship of relationships) {
    const source = positions.get(relationship.source);
    const target = positions.get(relationship.target);
    if (!source || !target) {
      continue;
    }
    const selected = state.selectedIdentifier &&
      (relationship.source === state.selectedIdentifier || relationship.target === state.selectedIdentifier);
    const dimmed = state.selectedIdentifier && !selected;
    const classes = ["dependency-map__edge"];
    if (relationship.testOnly) {
      classes.push("dependency-map__edge--test");
    }
    if (relationship.concerns.length > 0) {
      classes.push("dependency-map__edge--concern");
    }
    if (selected) {
      classes.push("dependency-map__edge--selected");
    }
    if (dimmed) {
      classes.push("dependency-map__edge--dimmed");
    }

    const path = createSvgElement("path", classes.join(" "));
    path.setAttribute("d", relationshipPath(source, target));
    path.setAttribute("marker-end", "url(#dependencyArrow)");
    const title = createSvgElement("title");
    const references = relationship.productionReferences + relationship.testReferences;
    const sourceComponent = componentByIdentifier(relationship.source);
    const targetComponent = componentByIdentifier(relationship.target);
    const stability = sourceComponent && targetComponent
      ? `\nI ${formatRatio(sourceComponent.instability)} → I ${formatRatio(targetComponent.instability)}`
      : "";
    const violation = relationship.stableDependencyViolation ? " · SDP violation" : "";
    title.textContent =
      `${relationship.source} → ${relationship.target}\n` +
      `${references} files · ${relationship.sourcePackages.length} source packages` +
      `${stability}${violation}`;
    path.append(title);
    content.append(path);
  }
}

function shortenedName(name) {
  const maximumLength = 25;
  if (name.length <= maximumLength) {
    return name;
  }
  return `${name.slice(0, maximumLength - 1)}…`;
}

function appendComponents(content, components, relationships, positions) {
  const connected = new Set();
  if (state.selectedIdentifier) {
    connected.add(state.selectedIdentifier);
    for (const relationship of relationships) {
      if (relationship.source === state.selectedIdentifier) {
        connected.add(relationship.target);
      }
      if (relationship.target === state.selectedIdentifier) {
        connected.add(relationship.source);
      }
    }
  }

  for (const component of components) {
    const position = positions.get(component.id);
    const classes = ["dependency-map__node"];
    if (component.id === state.selectedIdentifier) {
      classes.push("dependency-map__node--selected");
    }
    if (state.selectedIdentifier && !connected.has(component.id)) {
      classes.push("dependency-map__node--dimmed");
    }
    if (component.inCycle) {
      classes.push("dependency-map__node--cycle");
    }
    if (component.inZoneOfPain) {
      classes.push("dependency-map__node--pain");
    }

    const node = createSvgElement("g", classes.join(" "));
    node.dataset.kind = component.kind;
    node.setAttribute("transform", `translate(${position.x} ${position.y})`);
    node.setAttribute("tabindex", "0");
    node.setAttribute("role", "button");
    node.setAttribute("aria-label", `${component.name}, ${kindDefinition(component.kind).label}`);

    const box = createSvgElement("rect", "dependency-map__node-box");
    box.setAttribute("width", String(layoutOptions.nodeWidth));
    box.setAttribute("height", String(layoutOptions.nodeHeight));
    box.setAttribute("rx", "8");

    const kindDot = createSvgElement("circle", "dependency-map__node-kind");
    kindDot.setAttribute("cx", "14");
    kindDot.setAttribute("cy", "15");
    kindDot.setAttribute("r", "4");

    const name = createSvgElement("text", "dependency-map__node-name");
    name.setAttribute("x", "24");
    name.setAttribute("y", "18");
    name.textContent = shortenedName(component.name);

    const metric = createSvgElement("text", "dependency-map__node-metric");
    metric.setAttribute("x", "14");
    metric.setAttribute("y", "35");
    metric.textContent =
      `Cₐ ${component.afferentCoupling} · Cₑ ${component.efferentCoupling} · I ${formatRatio(component.instability)}`;

    const title = createSvgElement("title");
    title.textContent =
      `${component.id}\nCₐ ${component.afferentCoupling} · Cₑ ${component.efferentCoupling}\n` +
      `I ${formatRatio(component.instability)} · A ${formatRatio(component.abstractness)} · ` +
      `D ${formatRatio(component.mainSequenceDistance)}`;
    node.append(box, kindDot, name, metric, title);

    const hasConcern = component.inCycle || component.inZoneOfPain || relationships.some(
      (relationship) =>
        relationship.concerns.length > 0 &&
        (relationship.source === component.id || relationship.target === component.id),
    );
    if (hasConcern) {
      const alert = createSvgElement("circle", "dependency-map__node-alert");
      alert.setAttribute("cx", String(layoutOptions.nodeWidth - 11));
      alert.setAttribute("cy", "11");
      alert.setAttribute("r", "4");
      node.append(alert);
    }

    node.addEventListener("click", (event) => {
      event.stopPropagation();
      selectComponent(component.id, false);
    });
    node.addEventListener("keydown", (event) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        selectComponent(component.id, false);
      }
    });
    content.append(node);
  }
}

function renderGraph(shouldFit) {
  if (!state.graph) {
    return;
  }
  const components = graphComponents();
  const visibleIdentifiers = new Set(components.map((component) => component.id));
  const relationships = graphRelationships(visibleIdentifiers);
  const layout = layoutComponents(components);
  state.contentBounds = { width: layout.width, height: layout.height };
  elements.graphEmpty.hidden = components.length !== 0;
  elements.dependencyMap.replaceChildren();

  const definitions = createSvgElement("defs");
  appendArrowMarker(definitions);
  const content = createSvgElement("g", "dependency-map__content");
  appendGroupGuides(content, layout);
  appendRelationships(content, relationships, layout.positions);
  appendComponents(content, components, relationships, layout.positions);
  elements.dependencyMap.append(definitions, content);
  state.contentGroup = content;
  if (shouldFit) {
    fitGraph();
  } else {
    applyTransform();
  }
}

function renderViewState() {
  const impactVisible = state.viewMode === "impact";
  const metric = impactMetricDefinition();
  elements.impactView.setAttribute("aria-pressed", String(impactVisible));
  elements.dependencyView.setAttribute("aria-pressed", String(!impactVisible));
  elements.impactMetricControl.hidden = !impactVisible;
  elements.dependencyControls.hidden = impactVisible;
  elements.impactMap.hidden = !impactVisible;
  elements.dependencyMap.hidden = impactVisible;
  elements.impactLegend.hidden = !impactVisible;
  elements.edgeLegend.hidden = impactVisible;
  elements.graphViewport.dataset.view = state.viewMode;
  elements.graphViewport.dataset.dragging = "false";
  state.dragging = null;

  if (impactVisible) {
    elements.graphEyebrow.textContent = "Strategic weight";
    elements.graphTitle.textContent = "Dependency mass map";
    elements.graphDescription.textContent =
      `Area represents ${metric.label}. Larger components have more architectural weight.`;
    elements.impactLegendText.textContent = `Larger area means more ${metric.label}.`;
    elements.graphViewport.setAttribute(
      "aria-label",
      "Dependency impact map. Larger areas indicate greater architectural use.",
    );
    return;
  }

  elements.graphEyebrow.textContent = "Dependency flow";
  elements.graphTitle.textContent = "Component import map";
  elements.graphDescription.textContent =
    "Arrows point from an importer to the internal component that it imports.";
  elements.graphViewport.setAttribute(
    "aria-label",
    "Dependency graph. Drag to move. Use the wheel to zoom.",
  );
}

function renderActiveMap(shouldFit) {
  renderViewState();
  if (state.viewMode === "impact") {
    renderImpactMap();
    return;
  }
  renderGraph(shouldFit);
}

function setViewMode(viewMode) {
  if (viewMode !== "impact" && viewMode !== "dependencies") {
    return;
  }
  state.viewMode = viewMode;
  renderActiveMap(true);
}

function applyTransform() {
  if (!state.contentGroup) {
    return;
  }
  const { x, y, scale } = state.transform;
  state.contentGroup.setAttribute("transform", `translate(${x} ${y}) scale(${scale})`);
}

function fitGraph() {
  const viewportWidth = elements.graphViewport.clientWidth;
  const viewportHeight = elements.graphViewport.clientHeight;
  const contentWidth = state.contentBounds.width;
  const contentHeight = state.contentBounds.height;
  if (!contentWidth || !contentHeight || !viewportWidth || !viewportHeight) {
    return;
  }
  const horizontalScale = (viewportWidth - layoutOptions.fitPadding * 2) / contentWidth;
  const verticalScale = (viewportHeight - layoutOptions.fitPadding * 2) / contentHeight;
  const scale = Math.min(
    layoutOptions.maximumScale,
    Math.max(layoutOptions.minimumScale, Math.min(horizontalScale, verticalScale)),
  );
  state.transform = {
    scale,
    x: (viewportWidth - contentWidth * scale) / 2,
    y: (viewportHeight - contentHeight * scale) / 2,
  };
  applyTransform();
}

function zoomAt(factor, clientX, clientY) {
  const bounds = elements.graphViewport.getBoundingClientRect();
  const pointerX = clientX - bounds.left;
  const pointerY = clientY - bounds.top;
  const previousScale = state.transform.scale;
  const scale = Math.min(
    layoutOptions.maximumScale,
    Math.max(layoutOptions.minimumScale, previousScale * factor),
  );
  if (scale === previousScale) {
    return;
  }
  const contentX = (pointerX - state.transform.x) / previousScale;
  const contentY = (pointerY - state.transform.y) / previousScale;
  state.transform = {
    scale,
    x: pointerX - contentX * scale,
    y: pointerY - contentY * scale,
  };
  applyTransform();
}

function viewportCenter() {
  const bounds = elements.graphViewport.getBoundingClientRect();
  return { x: bounds.left + bounds.width / 2, y: bounds.top + bounds.height / 2 };
}

function componentByIdentifier(identifier) {
  return state.graph?.components.find((component) => component.id === identifier);
}

function relationshipVisibleInDetail(relationship) {
  return state.includeTests || !relationship.testOnly;
}

function relationshipButton(relationship, componentIdentifier, incoming) {
  const otherIdentifier = incoming ? relationship.source : relationship.target;
  const other = componentByIdentifier(otherIdentifier);
  const button = createElement("button", "detail__relation-button");
  button.type = "button";
  button.title = relationship.targetPackages.join(", ");
  button.addEventListener("click", () => selectComponent(otherIdentifier, true));
  const name = createElement("span", "detail__relation-name", other?.name || otherIdentifier);
  const references = relationship.productionReferences + relationship.testReferences;
  const suffix = relationship.testOnly
    ? " · test only"
    : relationship.stableDependencyViolation
      ? " · SDP violation"
      : "";
  const count = createElement("span", "detail__relation-count", `${references} files${suffix}`);
  button.append(name, count);
  return button;
}

function appendMetric(container, value, label) {
  const metric = createElement("div", "detail__metric");
  metric.append(
    createElement("strong", "detail__metric-value", String(value)),
    createElement("span", "detail__metric-label", label),
  );
  container.append(metric);
}

function appendDetailSection(container, title, content) {
  const section = createElement("section", "detail__section");
  section.append(createElement("h3", "detail__section-title", title), content);
  container.append(section);
}

function renderDetail() {
  const component = componentByIdentifier(state.selectedIdentifier);
  if (!component) {
    elements.detailPanel.replaceChildren();
    const empty = createElement("div", "detail__empty");
    const mark = createElement("span", "detail__empty-mark", "↗");
    mark.setAttribute("aria-hidden", "true");
    empty.append(
      mark,
      createElement("h2", "detail__empty-title", "Select a component"),
      createElement(
        "p",
        "detail__empty-copy",
        "Review its dependencies, consumers, and application impact.",
      ),
    );
    elements.detailPanel.append(empty);
    return;
  }

  elements.detailPanel.replaceChildren();
  const header = createElement("header", "detail__header");
  const heading = createElement("div", "detail__heading");
  const kind = createElement("span", "detail__kind", kindDefinition(component.kind).label);
  kind.dataset.kind = component.kind;
  heading.append(kind, createElement("h2", "detail__title", component.name));
  header.append(heading);
  elements.detailPanel.append(header, createElement("p", "detail__path", component.id));

  const metrics = createElement("div", "detail__metrics");
  appendMetric(metrics, component.afferentCoupling, "Cₐ · incoming");
  appendMetric(metrics, component.efferentCoupling, "Cₑ · outgoing");
  appendMetric(metrics, formatRatio(component.instability), "I · instability");
  appendMetric(metrics, formatRatio(component.abstractness), "A · abstractness");
  appendMetric(metrics, formatRatio(component.mainSequenceDistance), "D · main sequence");
  appendMetric(
    metrics,
    `${component.abstractTypes}/${component.abstractTypes + component.concreteTypes}`,
    "abstract types",
  );
  appendMetric(metrics, component.applicationReach, "applications");
  appendMetric(metrics, component.transitiveDependants, "indirect impact");
  appendMetric(metrics, component.sourceFiles, "Go files");
  elements.detailPanel.append(metrics);

  const stabilityFormula = createElement("div", "detail__formula");
  const totalCoupling = component.afferentCoupling + component.efferentCoupling;
  const instabilityFormula = totalCoupling === 0
    ? "I = 0.00 · no external production coupling"
    : `I = Cₑ / (Cₐ + Cₑ) = ${component.efferentCoupling} / ${totalCoupling} = ` +
      formatRatio(component.instability);
  const namedTypes = component.abstractTypes + component.concreteTypes;
  const abstractnessFormula = namedTypes === 0
    ? "A = 0.00 · no named production types"
    : `A = interfaces / named types = ${component.abstractTypes} / ${namedTypes} = ` +
      formatRatio(component.abstractness);
  stabilityFormula.append(
    createElement("code", "detail__formula-line", instabilityFormula),
    createElement("code", "detail__formula-line", abstractnessFormula),
    createElement(
      "code",
      "detail__formula-line",
      `D = |A + I − 1| = ${formatRatio(component.mainSequenceDistance)}`,
    ),
    createElement(
      "code",
      "detail__formula-line",
      `Pain zone = Cₐ > 0, I ≤ 0.20, A ≤ 0.20 · ${component.inZoneOfPain ? "yes" : "no"}`,
    ),
  );
  appendDetailSection(elements.detailPanel, "Stability calculation", stabilityFormula);

  if (component.applications.length > 0) {
    const applications = createElement("ul", "detail__application-list");
    for (const application of component.applications) {
      applications.append(createElement("li", "detail__application", application));
    }
    appendDetailSection(elements.detailPanel, "Application reach", applications);
  }

  const incoming = state.graph.relationships
    .filter(
      (relationship) =>
        relationship.target === component.id && relationshipVisibleInDetail(relationship),
    )
    .sort((first, second) => first.source.localeCompare(second.source));
  const incomingList = createElement("ul", "detail__relation-list");
  if (incoming.length === 0) {
    incomingList.append(createElement("li", "detail__none", "No component imports this component."));
  } else {
    for (const relationship of incoming) {
      const item = createElement("li", "detail__relation");
      item.append(relationshipButton(relationship, component.id, true));
      incomingList.append(item);
    }
  }
  appendDetailSection(elements.detailPanel, "Imported by", incomingList);

  const outgoing = state.graph.relationships
    .filter(
      (relationship) =>
        relationship.source === component.id && relationshipVisibleInDetail(relationship),
    )
    .sort((first, second) => first.target.localeCompare(second.target));
  const outgoingList = createElement("ul", "detail__relation-list");
  if (outgoing.length === 0) {
    outgoingList.append(
      createElement("li", "detail__none", "This component does not import another internal component."),
    );
  } else {
    for (const relationship of outgoing) {
      const item = createElement("li", "detail__relation");
      item.append(relationshipButton(relationship, component.id, false));
      outgoingList.append(item);
    }
  }
  appendDetailSection(elements.detailPanel, "Imports", outgoingList);

  const concerns = new Set();
  if (component.inCycle) {
    concerns.add("This component is in a production cycle.");
  }
  if (component.inZoneOfPain) {
    concerns.add(
      "Zone of Pain: production consumers depend on a stable component with little abstraction.",
    );
  }
  for (const relationship of [...incoming, ...outgoing]) {
    for (const concern of relationship.concerns) {
      concerns.add(concernLabels[concern] || concern);
    }
  }
  if (concerns.size > 0) {
    const concernList = createElement("ul", "detail__concern-list");
    for (const concern of [...concerns].sort()) {
      concernList.append(createElement("li", "detail__concern", concern));
    }
    appendDetailSection(elements.detailPanel, "Design alerts", concernList);
  }
}

function selectComponent(identifier, reveal) {
  const component = componentByIdentifier(identifier);
  if (!component) {
    return;
  }
  let filtersChanged = false;
  if (reveal && !state.visibleKinds.has(component.kind)) {
    state.visibleKinds.add(component.kind);
    filtersChanged = true;
  }
  if (reveal && state.search) {
    state.search = "";
    elements.searchInput.value = "";
    filtersChanged = true;
  }
  state.selectedIdentifier = identifier;
  if (filtersChanged) {
    renderKindControls();
  }
  renderActiveMap(filtersChanged);
  renderDetail();
}

function appendRankingEntry(list, component, position, value) {
  const item = createElement("li", "ranking__item");
  const button = createElement("button", "ranking__button");
  button.type = "button";
  button.dataset.kind = component.kind;
  button.title = component.id;
  button.addEventListener("click", () => selectComponent(component.id, true));
  button.append(
    createElement("span", "ranking__position", position),
    createElement("span", "ranking__name", component.name),
    createElement("span", "ranking__value", value),
  );
  item.append(button);
  list.append(item);
}

function renderRankings() {
  elements.usageRanking.replaceChildren();
  const reusableKinds = new Set([
    "application-module",
    "shared-module",
    "library",
    "infrastructure",
  ]);
  const used = state.graph.components
    .filter((component) => reusableKinds.has(component.kind) && component.productionDependants > 0)
    .sort(compareUsage)
    .slice(0, 7);
  if (used.length === 0) {
    elements.usageRanking.append(createElement("li", "detail__none", "No production relationship exists."));
  } else {
    used.forEach((component, index) => {
      appendRankingEntry(
        elements.usageRanking,
        component,
        String(index + 1).padStart(2, "0"),
        `${component.productionDependants} consumers`,
      );
    });
  }

  elements.unusedRanking.replaceChildren();
  const reviewableKinds = new Set(["application-module", "shared-module", "library"]);
  const unused = state.graph.components
    .filter((component) => reviewableKinds.has(component.kind) && component.productionDependants === 0)
    .sort(compareIdentifiers)
    .slice(0, 7);
  if (unused.length === 0) {
    elements.unusedRanking.append(
      createElement("li", "detail__none", "Each component has a production consumer."),
    );
  } else {
    unused.forEach((component, index) => {
      appendRankingEntry(
        elements.unusedRanking,
        component,
        String(index + 1).padStart(2, "0"),
        `${component.sourceFiles} files`,
      );
    });
  }

  elements.zoneOfPainRanking.replaceChildren();
  const painZone = state.graph.components
    .filter((component) => component.inZoneOfPain)
    .sort(
      (first, second) =>
        second.afferentCoupling - first.afferentCoupling ||
        second.mainSequenceDistance - first.mainSequenceDistance ||
        compareIdentifiers(first, second),
    )
    .slice(0, 7);
  if (painZone.length === 0) {
    elements.zoneOfPainRanking.append(
      createElement("li", "detail__none", "No component is in the stable concrete corner."),
    );
  } else {
    painZone.forEach((component, index) => {
      appendRankingEntry(
        elements.zoneOfPainRanking,
        component,
        String(index + 1).padStart(2, "0"),
        `Cₐ ${component.afferentCoupling} · I ${formatRatio(component.instability)}`,
      );
    });
  }
}

function renderDiagnostics() {
  const diagnostics = state.graph.diagnostics || [];
  elements.diagnostics.hidden = diagnostics.length === 0;
  elements.diagnosticsList.replaceChildren();
  if (diagnostics.length === 0) {
    return;
  }
  elements.diagnosticsSummary.textContent =
    `The analyzer could not fully analyze ${diagnostics.length} files`;
  for (const diagnostic of diagnostics) {
    elements.diagnosticsList.append(
      createElement(
        "li",
        "diagnostics__item",
        `${diagnostic.path}: ${diagnostic.message}`,
      ),
    );
  }
}

function renderDashboard() {
  renderSummary();
  renderKindControls();
  renderRankings();
  renderDiagnostics();
  renderActiveMap(true);
  renderDetail();
}

function setConnectionState(connectionState, label) {
  elements.connectionStatus.dataset.state = connectionState;
  elements.connectionLabel.textContent = label;
}

function requestGraph() {
  state.graphRequestPending = true;
  if (state.graphRequestRunning) {
    return state.graphRequestPromise;
  }
  state.graphRequestRunning = true;
  state.graphRequestPromise = (async () => {
    try {
      do {
        state.graphRequestPending = false;
        const response = await fetch("/api/graph", { cache: "no-store" });
        if (!response.ok) {
          throw new Error(`HTTP status ${response.status}`);
        }
        const graph = await response.json();
        if (state.graph?.revision !== graph.revision) {
          state.graph = graph;
          if (state.selectedIdentifier && !componentByIdentifier(state.selectedIdentifier)) {
            state.selectedIdentifier = "";
          }
          renderDashboard();
        }
      } while (state.graphRequestPending);
      return true;
    } catch (error) {
      setConnectionState("error", "Read error");
      elements.revisionLabel.textContent = String(error);
      return false;
    } finally {
      state.graphRequestRunning = false;
      state.graphRequestPromise = null;
    }
  })();
  return state.graphRequestPromise;
}

function connectEventStream() {
  const events = new EventSource("/api/events");
  events.addEventListener("ready", (event) => {
    state.eventStreamConnected = true;
    setConnectionState("live", "Live");
    if (state.graph?.revision !== event.data) {
      requestGraph();
    }
  });
  events.addEventListener("graph", () => {
    state.eventStreamConnected = true;
    setConnectionState("live", "Updating");
    requestGraph().then((succeeded) => {
      if (succeeded && state.eventStreamConnected) {
        setConnectionState("live", "Live");
      }
    });
  });
  events.onerror = () => {
    state.eventStreamConnected = false;
    setConnectionState("connecting", "Reconnecting");
  };
}

elements.searchInput.addEventListener("input", () => {
  state.search = normalizeSearch(elements.searchInput.value);
  renderActiveMap(true);
});

elements.testToggle.addEventListener("change", () => {
  state.includeTests = elements.testToggle.checked;
  renderActiveMap(false);
  renderDetail();
});

elements.impactView.addEventListener("click", () => setViewMode("impact"));
elements.dependencyView.addEventListener("click", () => setViewMode("dependencies"));

elements.impactMetric.addEventListener("change", () => {
  if (!impactMetricDefinitions[elements.impactMetric.value]) {
    return;
  }
  state.impactMetric = elements.impactMetric.value;
  renderActiveMap(false);
});

elements.zoomIn.addEventListener("click", () => {
  const center = viewportCenter();
  zoomAt(1.25, center.x, center.y);
});

elements.zoomOut.addEventListener("click", () => {
  const center = viewportCenter();
  zoomAt(0.8, center.x, center.y);
});

elements.fitGraph.addEventListener("click", fitGraph);

elements.graphViewport.addEventListener(
  "wheel",
  (event) => {
    if (state.viewMode !== "dependencies") {
      return;
    }
    event.preventDefault();
    zoomAt(event.deltaY < 0 ? 1.12 : 0.89, event.clientX, event.clientY);
  },
  { passive: false },
);

elements.graphViewport.addEventListener("pointerdown", (event) => {
  if (state.viewMode !== "dependencies") {
    return;
  }
  if (event.target.closest?.(".dependency-map__node")) {
    return;
  }
  elements.graphViewport.setPointerCapture(event.pointerId);
  elements.graphViewport.dataset.dragging = "true";
  state.dragging = {
    pointerIdentifier: event.pointerId,
    clientX: event.clientX,
    clientY: event.clientY,
    originX: state.transform.x,
    originY: state.transform.y,
  };
});

elements.graphViewport.addEventListener("pointermove", (event) => {
  if (!state.dragging || state.dragging.pointerIdentifier !== event.pointerId) {
    return;
  }
  state.transform.x = state.dragging.originX + event.clientX - state.dragging.clientX;
  state.transform.y = state.dragging.originY + event.clientY - state.dragging.clientY;
  applyTransform();
});

function finishDragging(event) {
  if (!state.dragging || state.dragging.pointerIdentifier !== event.pointerId) {
    return;
  }
  state.dragging = null;
  elements.graphViewport.dataset.dragging = "false";
  if (elements.graphViewport.hasPointerCapture(event.pointerId)) {
    elements.graphViewport.releasePointerCapture(event.pointerId);
  }
}

elements.graphViewport.addEventListener("pointerup", finishDragging);
elements.graphViewport.addEventListener("pointercancel", finishDragging);

document.addEventListener("keydown", (event) => {
  const targetName = event.target?.tagName?.toLocaleLowerCase("en-GB");
  if (event.key === "/" && targetName !== "input" && targetName !== "textarea") {
    event.preventDefault();
    elements.searchInput.focus();
  }
  if (event.key === "Escape" && document.activeElement === elements.searchInput) {
    elements.searchInput.value = "";
    state.search = "";
    elements.searchInput.blur();
    renderActiveMap(true);
  }
});

new ResizeObserver(() => {
  if (state.graph) {
    if (state.viewMode === "impact") {
      renderImpactMap();
    } else {
      applyTransform();
    }
  }
}).observe(elements.graphViewport);

setConnectionState("connecting", "Connecting");
requestGraph();
connectEventStream();
