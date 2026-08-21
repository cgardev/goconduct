"use strict";

const svgNamespace = "http://www.w3.org/2000/svg";
const numberFormatter = new Intl.NumberFormat("en-GB");
const themeStorageKey = "dependencygraph-theme";
const supportedThemes = new Set(["light", "dark"]);

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

const ruleViolationLabels = {
  "production-imports-development": "Production code imports development code.",
  "library-imports-feature": "A shared library imports a feature module.",
  "shared-component-imports-application": "A shared component imports application code.",
  "cross-application-module-import":
    "An application module imports a module from a different application.",
  "stable-dependency-principle":
    "Stable dependency principle violation: this component imports a less stable component.",
};

const componentMetricDefinitions = Object.freeze({
  importingComponents: {
    field: "afferentCoupling",
    label: "direct production importing components, or afferent coupling (Cₐ)",
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
    field: "transitiveImportingComponents",
    label: "transitive production importing components",
    shortLabel: "importers",
  },
  files: {
    field: "sourceFiles",
    label: "Go source files",
    shortLabel: "Go files",
  },
});

const functionMetricDefinitions = Object.freeze({
  incomingCalls: {
    label: "incoming call sites",
    shortLabel: "calls in",
  },
  afferent: {
    label: "caller functions, or afferent coupling (Cₐ)",
    shortLabel: "Cₐ",
  },
  efferent: {
    label: "called functions, or efferent coupling (Cₑ)",
    shortLabel: "Cₑ",
  },
  outgoingCalls: {
    label: "outgoing call sites",
    shortLabel: "calls out",
  },
  transitiveCallers: {
    label: "transitive caller functions",
    shortLabel: "callers",
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
  viewportPadding: 28,
  minimumScale: 0.12,
  maximumScale: 2.4,
});

const componentMetricMapOptions = Object.freeze({
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
  summaryFunctions: document.getElementById("summaryFunctions"),
  summaryFunctionCalls: document.getElementById("summaryFunctionCalls"),
  summaryUsingApplications: document.getElementById("summaryUsingApplications"),
  summaryUsingApplicationsNote: document.getElementById("summaryUsingApplicationsNote"),
  summaryFindings: document.getElementById("summaryFindings"),
  summaryCycles: document.getElementById("summaryCycles"),
  searchInput: document.getElementById("searchInput"),
  kindFilters: document.getElementById("kindFilters"),
  testToggle: document.getElementById("testToggle"),
  graphEyebrow: document.getElementById("graphEyebrow"),
  graphTitle: document.getElementById("graphTitle"),
  graphDescription: document.getElementById("graphDescription"),
  graphViewport: document.getElementById("graphViewport"),
  componentMetricMap: document.getElementById("componentMetricMap"),
  functionMetricMap: document.getElementById("functionMetricMap"),
  dependencyMap: document.getElementById("dependencyMap"),
  graphEmpty: document.getElementById("graphEmpty"),
  legend: document.getElementById("legend"),
  componentMetricView: document.getElementById("componentMetricView"),
  functionMetricView: document.getElementById("functionMetricView"),
  dependencyView: document.getElementById("dependencyView"),
  componentMetricControl: document.getElementById("componentMetricControl"),
  componentMetric: document.getElementById("componentMetric"),
  functionMetricControl: document.getElementById("functionMetricControl"),
  functionMetric: document.getElementById("functionMetric"),
  dependencyControls: document.getElementById("dependencyControls"),
  componentMetricLegend: document.getElementById("componentMetricLegend"),
  componentMetricLegendText: document.getElementById("componentMetricLegendText"),
  edgeLegend: document.getElementById("edgeLegend"),
  zoomOut: document.getElementById("zoomOut"),
  showAllComponents: document.getElementById("showAllComponents"),
  zoomIn: document.getElementById("zoomIn"),
  detailPanel: document.getElementById("detailPanel"),
  usageRanking: document.getElementById("usageRanking"),
  functionUsageRanking: document.getElementById("functionUsageRanking"),
  unusedRanking: document.getElementById("unusedRanking"),
  stableLowAbstractionRanking: document.getElementById("stableLowAbstractionRanking"),
  diagnostics: document.getElementById("diagnostics"),
  diagnosticsSummary: document.getElementById("diagnosticsSummary"),
  diagnosticsList: document.getElementById("diagnosticsList"),
  lightTheme: document.getElementById("lightTheme"),
  darkTheme: document.getElementById("darkTheme"),
};

const state = {
  graph: null,
  selectedIdentifier: "",
  selectedFunctionIdentifier: "",
  visibleKinds: new Set(kindDefinitions.map((definition) => definition.identifier)),
  includeTests: true,
  search: "",
  viewMode: "metrics",
  componentMetric: "importingComponents",
  functionMetric: "incomingCalls",
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

function storedTheme() {
  try {
    const theme = window.localStorage.getItem(themeStorageKey);
    return supportedThemes.has(theme) ? theme : "dark";
  } catch {
    return "dark";
  }
}

function setTheme(theme, persist) {
  const selectedTheme = supportedThemes.has(theme) ? theme : "dark";
  document.documentElement.dataset.theme = selectedTheme;
  elements.lightTheme.setAttribute("aria-pressed", String(selectedTheme === "light"));
  elements.darkTheme.setAttribute("aria-pressed", String(selectedTheme === "dark"));
  if (!persist) {
    return;
  }
  try {
    window.localStorage.setItem(themeStorageKey, selectedTheme);
  } catch {
    // The selected theme still applies when browser storage is unavailable.
  }
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

function compareByImportingComponents(first, second) {
  return (
    second.productionImportingComponents - first.productionImportingComponents ||
    second.productionImporterPackages - first.productionImporterPackages ||
    second.usingApplicationCount - first.usingApplicationCount ||
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
    ...(component.usingApplications || []),
  ].join(" ");
  return normalizeSearch(searchable).includes(state.search);
}

function visibleComponents() {
  if (!state.graph) {
    return [];
  }
  return state.graph.components.filter(
    (component) => state.visibleKinds.has(component.kind) && componentMatchesSearch(component),
  );
}

function visibleRelationships(visibleIdentifiers) {
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

function functionMatchesSearch(functionValue) {
  if (!state.search) {
    return true;
  }
  const searchable = [
    functionValue.id,
    functionValue.name,
    functionValue.package,
    functionValue.component,
    functionValue.path,
  ].join(" ");
  return normalizeSearch(searchable).includes(state.search);
}

function visibleFunctions() {
  if (!state.graph) {
    return [];
  }
  return state.graph.functions.filter((functionValue) => {
    const component = componentByIdentifier(functionValue.component);
    return component &&
      state.visibleKinds.has(component.kind) &&
      (state.includeTests || !functionValue.test) &&
      functionMatchesSearch(functionValue);
  });
}

function selectedComponentMetric() {
  return componentMetricDefinitions[state.componentMetric] || componentMetricDefinitions.importingComponents;
}

function selectedMetricValue(component) {
  const value = Number(component[selectedComponentMetric().field]);
  return Number.isFinite(value) && value > 0 ? value : 0;
}

function compareBySelectedMetric(first, second) {
  return selectedMetricValue(second) - selectedMetricValue(first) ||
    compareByImportingComponents(first, second);
}

function selectedFunctionMetric() {
  return functionMetricDefinitions[state.functionMetric] || functionMetricDefinitions.incomingCalls;
}

function selectedFunctionMetricValue(functionValue) {
  const testIncoming = state.includeTests ? functionValue.testIncomingCallSites : 0;
  const testOutgoing = state.includeTests ? functionValue.testOutgoingCallSites : 0;
  const testAfferent = state.includeTests ? functionValue.testAfferentCoupling : 0;
  const testEfferent = state.includeTests ? functionValue.testEfferentCoupling : 0;
  switch (state.functionMetric) {
    case "afferent":
      return functionValue.afferentCoupling + testAfferent;
    case "efferent":
      return functionValue.efferentCoupling + testEfferent;
    case "outgoingCalls":
      return functionValue.outgoingCallSites + testOutgoing;
    case "transitiveCallers":
      return functionValue.transitiveCallerFunctions;
    default:
      return functionValue.incomingCallSites + testIncoming;
  }
}

function compareBySelectedFunctionMetric(first, second) {
  return selectedFunctionMetricValue(second) - selectedFunctionMetricValue(first) ||
    first.id.localeCompare(second.id);
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

function shortenedMetricName(name, width) {
  const maximumLength = Math.max(5, Math.floor((width - 20) / 7));
  if (name.length <= maximumLength) {
    return name;
  }
  return `${name.slice(0, Math.max(1, maximumLength - 1))}…`;
}

function appendMetricTile(container, tile, rankingPosition, identifiersWithRuleViolations) {
  const { component, value, width, height } = tile;
  const inset = Math.min(
    componentMetricMapOptions.gap / 2,
    Math.max(0, width / 5),
    Math.max(0, height / 5),
  );
  const contentWidth = Math.max(0.5, width - inset * 2);
  const contentHeight = Math.max(0.5, height - inset * 2);
  const rankVisible = contentWidth >= 145 && contentHeight >= 32;
  const classes = ["component-metric-map__tile"];
  if (component.id === state.selectedIdentifier) {
    classes.push("component-metric-map__tile--selected");
  }
  if (value === 0) {
    classes.push("component-metric-map__tile--zero");
  }
  if (component.inCycle) {
    classes.push("component-metric-map__tile--cycle");
  }
  if (component.isStableWithLowAbstraction) {
    classes.push("component-metric-map__tile--stable-low-abstraction");
  }

  const node = createSvgElement("g", classes.join(" "));
  node.dataset.kind = component.kind;
  node.setAttribute("transform", `translate(${tile.x} ${tile.y})`);
  node.setAttribute("tabindex", "0");
  node.setAttribute("role", "button");
  node.setAttribute(
    "aria-label",
    `${component.name}, ${numberFormatter.format(value)} ${selectedComponentMetric().label}`,
  );

  const box = createSvgElement("rect", "component-metric-map__tile-box");
  box.setAttribute("x", String(inset));
  box.setAttribute("y", String(inset));
  box.setAttribute("width", String(contentWidth));
  box.setAttribute("height", String(contentHeight));
  box.setAttribute("rx", String(Math.min(6, contentWidth / 5, contentHeight / 5)));
  node.append(box);

  const title = createSvgElement("title");
  title.textContent =
    `${component.id}\nAfferent coupling (Cₐ): ${component.afferentCoupling} · ` +
    `Efferent coupling (Cₑ): ${component.efferentCoupling}\n` +
    `Instability (I): ${formatRatio(component.instability)} · ` +
    `Abstractness (A): ${formatRatio(component.abstractness)}\n` +
    `Main-sequence distance (D): ${formatRatio(component.mainSequenceDistance)}\n` +
    `${component.abstractTypes} abstract types · ${component.concreteTypes} concrete types`;
  node.append(title);

  if (contentWidth >= 72 && contentHeight >= 32) {
    const name = createSvgElement("text", "component-metric-map__tile-name");
    name.setAttribute("x", String(inset + 10));
    name.setAttribute("y", String(inset + 18));
    name.textContent = shortenedMetricName(component.name, contentWidth - (rankVisible ? 42 : 0));
    node.append(name);
  }
  if (rankVisible) {
    const rankLabel = createSvgElement("text", "component-metric-map__tile-rank");
    rankLabel.setAttribute("x", String(width - inset - 10));
    rankLabel.setAttribute("y", String(inset + 17));
    rankLabel.setAttribute("text-anchor", "end");
    rankLabel.textContent = `#${String(rankingPosition).padStart(2, "0")}`;
    node.append(rankLabel);
  }
  if (contentWidth >= 88 && contentHeight >= 52) {
    const metric = createSvgElement("text", "component-metric-map__tile-value");
    metric.setAttribute("x", String(inset + 10));
    metric.setAttribute("y", String(inset + 38));
    metric.textContent =
      `${numberFormatter.format(value)} ${selectedComponentMetric().shortLabel}`;
    node.append(metric);
  }
  if (contentWidth >= 122 && contentHeight >= 76) {
    const kind = createSvgElement("text", "component-metric-map__tile-kind");
    kind.setAttribute("x", String(inset + 10));
    kind.setAttribute("y", String(height - inset - 10));
    kind.textContent = kindDefinition(component.kind).label;
    node.append(kind);
  }
  if (
    (identifiersWithRuleViolations.has(component.id) || component.isStableWithLowAbstraction) &&
    contentWidth >= 24 &&
    contentHeight >= 24
  ) {
    const alert = createSvgElement("circle", "component-metric-map__tile-alert");
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

function renderComponentMetricMap() {
  if (!state.graph) {
    return;
  }
  const components = visibleComponents().sort(compareBySelectedMetric);
  elements.graphEmpty.hidden = components.length !== 0;
  elements.componentMetricMap.replaceChildren();

  const width = Math.max(elements.graphViewport.clientWidth, componentMetricMapOptions.minimumWidth);
  const height = Math.max(elements.graphViewport.clientHeight, componentMetricMapOptions.minimumHeight);
  elements.componentMetricMap.setAttribute("viewBox", `0 0 ${width} ${height}`);

  const weightedComponents = components.map((component) => {
    const value = selectedMetricValue(component);
    return {
      component,
      value,
      weight: value > 0 ? value : componentMetricMapOptions.zeroWeight,
    };
  });
  const padding = componentMetricMapOptions.padding;
  const tiles = layoutTreemap(weightedComponents, {
    x: padding,
    y: padding,
    width: width - padding * 2,
    height: height - padding * 2,
  });
  const identifiersWithRuleViolations = new Set();
  for (const relationship of state.graph.relationships) {
    if (relationship.ruleViolations.length > 0) {
      identifiersWithRuleViolations.add(relationship.source);
      identifiersWithRuleViolations.add(relationship.target);
    }
  }
  tiles.forEach((tile, index) => {
    appendMetricTile(elements.componentMetricMap, tile, index + 1, identifiersWithRuleViolations);
  });
}

function appendFunctionMetricTile(container, tile, rankingPosition) {
  const { functionValue, value, width, height } = tile;
  const component = componentByIdentifier(functionValue.component);
  const inset = Math.min(
    componentMetricMapOptions.gap / 2,
    Math.max(0, width / 5),
    Math.max(0, height / 5),
  );
  const contentWidth = Math.max(0.5, width - inset * 2);
  const contentHeight = Math.max(0.5, height - inset * 2);
  const rankVisible = contentWidth >= 145 && contentHeight >= 32;
  const classes = ["component-metric-map__tile"];
  if (functionValue.id === state.selectedFunctionIdentifier) {
    classes.push("component-metric-map__tile--selected");
  }
  if (value === 0) {
    classes.push("component-metric-map__tile--zero");
  }
  if (functionValue.inCycle) {
    classes.push("component-metric-map__tile--cycle");
  }

  const node = createSvgElement("g", classes.join(" "));
  node.dataset.kind = component?.kind || "infrastructure";
  node.setAttribute("transform", `translate(${tile.x} ${tile.y})`);
  node.setAttribute("tabindex", "0");
  node.setAttribute("role", "button");
  node.setAttribute(
    "aria-label",
    `${functionValue.name}, ${numberFormatter.format(value)} ${selectedFunctionMetric().label}`,
  );

  const box = createSvgElement("rect", "component-metric-map__tile-box");
  box.setAttribute("x", String(inset));
  box.setAttribute("y", String(inset));
  box.setAttribute("width", String(contentWidth));
  box.setAttribute("height", String(contentHeight));
  box.setAttribute("rx", String(Math.min(6, contentWidth / 5, contentHeight / 5)));
  node.append(box);

  const title = createSvgElement("title");
  title.textContent =
    `${functionValue.id}\nIncoming call sites: ${functionValue.incomingCallSites} · ` +
    `Outgoing call sites: ${functionValue.outgoingCallSites}\n` +
    `Afferent coupling (Cₐ): ${functionValue.afferentCoupling} · ` +
    `Efferent coupling (Cₑ): ${functionValue.efferentCoupling}\n` +
    `Instability (I): ${formatRatio(functionValue.instability)}`;
  node.append(title);

  if (contentWidth >= 72 && contentHeight >= 32) {
    const name = createSvgElement("text", "component-metric-map__tile-name");
    name.setAttribute("x", String(inset + 10));
    name.setAttribute("y", String(inset + 18));
    name.textContent = shortenedMetricName(
      functionValue.name,
      contentWidth - (rankVisible ? 42 : 0),
    );
    node.append(name);
  }
  if (rankVisible) {
    const rank = createSvgElement("text", "component-metric-map__tile-rank");
    rank.setAttribute("x", String(width - inset - 10));
    rank.setAttribute("y", String(inset + 17));
    rank.setAttribute("text-anchor", "end");
    rank.textContent = `#${String(rankingPosition).padStart(2, "0")}`;
    node.append(rank);
  }
  if (contentWidth >= 88 && contentHeight >= 52) {
    const metric = createSvgElement("text", "component-metric-map__tile-value");
    metric.setAttribute("x", String(inset + 10));
    metric.setAttribute("y", String(inset + 38));
    metric.textContent = `${numberFormatter.format(value)} ${selectedFunctionMetric().shortLabel}`;
    node.append(metric);
  }
  if (contentWidth >= 122 && contentHeight >= 76) {
    const componentName = createSvgElement("text", "component-metric-map__tile-kind");
    componentName.setAttribute("x", String(inset + 10));
    componentName.setAttribute("y", String(height - inset - 10));
    componentName.textContent = component?.name || functionValue.component;
    node.append(componentName);
  }

  node.addEventListener("click", (event) => {
    event.stopPropagation();
    selectFunction(functionValue.id, false);
  });
  node.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      selectFunction(functionValue.id, false);
    }
  });
  container.append(node);
}

function renderFunctionMetricMap() {
  if (!state.graph) {
    return;
  }
  const functions = visibleFunctions().sort(compareBySelectedFunctionMetric);
  elements.graphEmpty.hidden = functions.length !== 0;
  elements.functionMetricMap.replaceChildren();

  const width = Math.max(elements.graphViewport.clientWidth, componentMetricMapOptions.minimumWidth);
  const height = Math.max(elements.graphViewport.clientHeight, componentMetricMapOptions.minimumHeight);
  elements.functionMetricMap.setAttribute("viewBox", `0 0 ${width} ${height}`);
  const weightedFunctions = functions.map((functionValue) => {
    const value = selectedFunctionMetricValue(functionValue);
    return {
      functionValue,
      value,
      weight: value > 0 ? value : componentMetricMapOptions.zeroWeight,
    };
  });
  const padding = componentMetricMapOptions.padding;
  const tiles = layoutTreemap(weightedFunctions, {
    x: padding,
    y: padding,
    width: width - padding * 2,
    height: height - padding * 2,
  });
  tiles.forEach((tile, index) => {
    appendFunctionMetricTile(elements.functionMetricMap, tile, index + 1);
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
  const analysisPaths = state.graph.scope?.paths?.join(", ") || ".";
  elements.modulePath.textContent = `${state.graph.modulePath} · scope ${analysisPaths}`;
  elements.summaryComponents.textContent = numberFormatter.format(summary.components);
  elements.summaryKinds.textContent =
    `${summary.applications} applications · ` +
    `${summary.sharedModules + summary.applicationModules} modules · ` +
    `${summary.libraries} libraries`;
  elements.summaryRelationships.textContent = numberFormatter.format(summary.productionRelationships);
  elements.summaryTests.textContent = `${summary.testOnlyRelationships} test-only relationships`;
  elements.summaryFunctions.textContent = numberFormatter.format(summary.productionFunctions);
  elements.summaryFunctionCalls.textContent =
    `${numberFormatter.format(summary.functionCallSites)} resolved call sites · ` +
    `${numberFormatter.format(summary.testFunctions)} test functions · ` +
    `${numberFormatter.format(summary.functionCycles)} call cycles`;

  const sharedComponents = components
    .filter((component) => !["application", "development"].includes(component.kind))
    .sort(
      (first, second) =>
        second.usingApplicationCount - first.usingApplicationCount ||
        second.transitiveImportingComponents - first.transitiveImportingComponents ||
        compareByImportingComponents(first, second),
    );
  const componentUsedByMostApplications = sharedComponents[0];
  elements.summaryUsingApplications.textContent = componentUsedByMostApplications?.name || "—";
  elements.summaryUsingApplications.title = componentUsedByMostApplications?.id || "";
  elements.summaryUsingApplicationsNote.textContent = componentUsedByMostApplications
    ? `${componentUsedByMostApplications.usingApplicationCount} applications · ` +
      `${componentUsedByMostApplications.transitiveImportingComponents} transitive importing components`
    : "No shared component";

  elements.summaryFindings.textContent = numberFormatter.format(
    summary.findings,
  );
  elements.summaryCycles.textContent =
    `${summary.errors} errors · ${summary.warnings} warnings · ` +
    `${summary.stableDependencyPrincipleViolations} stable dependency principle violations`;
  elements.revisionLabel.textContent = `graph revision ${state.graph.revision.slice(0, 12)}`;
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
    if (relationship.ruleViolations.length > 0) {
      classes.push("dependency-map__edge--rule-violation");
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
      ? `\nInstability (I): ${formatRatio(sourceComponent.instability)} → ` +
        `${formatRatio(targetComponent.instability)}`
      : "";
    const violation = relationship.violatesStableDependencyPrinciple
      ? " · stable dependency principle violation"
      : "";
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
    if (component.isStableWithLowAbstraction) {
      classes.push("dependency-map__node--stable-low-abstraction");
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
      `Incoming ${component.afferentCoupling} · outgoing ${component.efferentCoupling} · ` +
      `instability ${formatRatio(component.instability)}`;

    const title = createSvgElement("title");
    title.textContent =
      `${component.id}\nAfferent coupling (Cₐ): ${component.afferentCoupling} · ` +
      `Efferent coupling (Cₑ): ${component.efferentCoupling}\n` +
      `Instability (I): ${formatRatio(component.instability)} · ` +
      `Abstractness (A): ${formatRatio(component.abstractness)} · ` +
      `Main-sequence distance (D): ${formatRatio(component.mainSequenceDistance)}`;
    node.append(box, kindDot, name, metric, title);

    const hasFinding = component.inCycle || component.isStableWithLowAbstraction || relationships.some(
      (relationship) =>
        relationship.ruleViolations.length > 0 &&
        (relationship.source === component.id || relationship.target === component.id),
    );
    if (hasFinding) {
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

function renderGraph(shouldShowAllComponents) {
  if (!state.graph) {
    return;
  }
  const components = visibleComponents();
  const visibleIdentifiers = new Set(components.map((component) => component.id));
  const relationships = visibleRelationships(visibleIdentifiers);
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
  if (shouldShowAllComponents) {
    showAllComponents();
  } else {
    applyTransform();
  }
}

function renderViewState() {
  const componentMapVisible = state.viewMode === "metrics";
  const functionMapVisible = state.viewMode === "functions";
  const dependencyMapVisible = state.viewMode === "dependencies";
  elements.componentMetricView.setAttribute("aria-pressed", String(componentMapVisible));
  elements.functionMetricView.setAttribute("aria-pressed", String(functionMapVisible));
  elements.dependencyView.setAttribute("aria-pressed", String(dependencyMapVisible));
  elements.componentMetricControl.hidden = !componentMapVisible;
  elements.functionMetricControl.hidden = !functionMapVisible;
  elements.dependencyControls.hidden = !dependencyMapVisible;
  elements.componentMetricMap.hidden = !componentMapVisible;
  elements.functionMetricMap.hidden = !functionMapVisible;
  elements.dependencyMap.hidden = !dependencyMapVisible;
  elements.componentMetricLegend.hidden = dependencyMapVisible;
  elements.edgeLegend.hidden = !dependencyMapVisible;
  elements.graphViewport.dataset.view = state.viewMode;
  elements.graphViewport.dataset.dragging = "false";
  state.dragging = null;

  if (componentMapVisible) {
    const selectedMetric = selectedComponentMetric();
    elements.graphEyebrow.textContent = "Component import count";
    elements.graphTitle.textContent = "Component usage map";
    elements.graphDescription.textContent =
      `The tile area represents ${selectedMetric.label}. A larger area means a larger value.`;
    elements.componentMetricLegendText.textContent =
      `A larger area means more ${selectedMetric.label}.`;
    elements.graphViewport.setAttribute(
      "aria-label",
      "Component count map. A larger area means a larger value.",
    );
    return;
  }

  if (functionMapVisible) {
    const selectedMetric = selectedFunctionMetric();
    elements.graphEyebrow.textContent = "Resolved static function calls";
    elements.graphTitle.textContent = "Function usage map";
    elements.graphDescription.textContent =
      `The tile area represents ${selectedMetric.label}. ` +
      "Go static type information selects each target.";
    elements.componentMetricLegendText.textContent =
      `A larger area means more ${selectedMetric.label}.`;
    elements.graphViewport.setAttribute(
      "aria-label",
      "Function count map. A larger area means a larger value.",
    );
    return;
  }

  elements.graphEyebrow.textContent = "Import relationships";
  elements.graphTitle.textContent = "Component import map";
  elements.graphDescription.textContent =
    "Arrows point from an importer to the internal component that it imports.";
  elements.graphViewport.setAttribute(
    "aria-label",
    "Dependency graph. Drag to move. Use the wheel to zoom.",
  );
}

function renderActiveMap(shouldShowAllComponents) {
  renderViewState();
  if (state.viewMode === "metrics") {
    renderComponentMetricMap();
    return;
  }
  if (state.viewMode === "functions") {
    renderFunctionMetricMap();
    return;
  }
  renderGraph(shouldShowAllComponents);
}

function setViewMode(viewMode) {
  if (!["metrics", "functions", "dependencies"].includes(viewMode)) {
    return;
  }
  state.viewMode = viewMode;
  renderActiveMap(true);
  renderDetail();
}

function applyTransform() {
  if (!state.contentGroup) {
    return;
  }
  const { x, y, scale } = state.transform;
  state.contentGroup.setAttribute("transform", `translate(${x} ${y}) scale(${scale})`);
}

function showAllComponents() {
  const viewportWidth = elements.graphViewport.clientWidth;
  const viewportHeight = elements.graphViewport.clientHeight;
  const contentWidth = state.contentBounds.width;
  const contentHeight = state.contentBounds.height;
  if (!contentWidth || !contentHeight || !viewportWidth || !viewportHeight) {
    return;
  }
  const horizontalScale = (viewportWidth - layoutOptions.viewportPadding * 2) / contentWidth;
  const verticalScale = (viewportHeight - layoutOptions.viewportPadding * 2) / contentHeight;
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

function functionByIdentifier(identifier) {
  return state.graph?.functions.find((functionValue) => functionValue.id === identifier);
}

function relationshipVisibleInDetail(relationship) {
  return state.includeTests || !relationship.testOnly;
}

function relationshipButton(relationship, sourceImportsSelectedComponent) {
  const otherIdentifier = sourceImportsSelectedComponent ? relationship.source : relationship.target;
  const other = componentByIdentifier(otherIdentifier);
  const button = createElement("button", "detail__relation-button");
  button.type = "button";
  button.title = relationship.targetPackages.join(", ");
  button.addEventListener("click", () => selectComponent(otherIdentifier, true));
  const name = createElement("span", "detail__relation-name", other?.name || otherIdentifier);
  const references = relationship.productionReferences + relationship.testReferences;
  const suffix = relationship.testOnly
    ? " · test only"
    : relationship.violatesStableDependencyPrinciple
      ? " · stable dependency principle violation"
      : "";
  const functionCallSites =
    relationship.productionFunctionCallSites + relationship.testFunctionCallSites;
  const callText = functionCallSites > 0 ? ` · ${functionCallSites} calls` : "";
  const count = createElement(
    "span",
    "detail__relation-count",
    `${references} files${callText}${suffix}`,
  );
  button.append(name, count);
  return button;
}

function appendRelationshipEvidence(container, relationship) {
  const calls = state.graph.functionCalls.filter(
    (call) =>
      call.sourceComponent === relationship.source &&
      call.targetComponent === relationship.target &&
      (state.includeTests || !call.testOnly),
  );
  const imports = (relationship.importSites || []).filter(
    (site) => state.includeTests || !site.test,
  );
  const callSiteCount = calls.reduce((total, call) => total + call.calls, 0);
  const evidence = createElement("details", "detail__evidence");
  const summary = createElement(
    "summary",
    "detail__evidence-summary",
    `${callSiteCount} resolved call sites · ${imports.length} import declarations`,
  );
  const evidenceList = createElement("ul", "detail__call-list");
  for (const call of calls) {
    for (const site of call.callSites) {
      evidenceList.append(
        createElement(
          "li",
          "detail__call",
          `${call.source} → ${call.target} · ${site.path}:${site.line}:${site.column}`,
        ),
      );
    }
  }
  for (const site of imports) {
    const alias = site.alias ? ` as ${site.alias}` : "";
    evidenceList.append(
      createElement(
        "li",
        "detail__call",
        `import ${site.targetPackage}${alias} · ${site.path}:${site.line}`,
      ),
    );
  }
  if (evidenceList.childElementCount === 0) {
    evidenceList.append(createElement("li", "detail__none", "No visible source evidence."));
  }
  evidence.append(summary, evidenceList);
  container.append(evidence);
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

function appendFunctionCallList(container, calls, incoming) {
  if (calls.length === 0) {
    container.append(
      createElement(
        "li",
        "detail__none",
        incoming ? "No visible function calls this function." : "This function has no visible calls.",
      ),
    );
    return;
  }
  for (const call of calls) {
    const otherIdentifier = incoming ? call.source : call.target;
    const item = createElement("li", "detail__relation");
    const button = createElement("button", "detail__function-button");
    button.type = "button";
    button.title = otherIdentifier;
    button.addEventListener("click", () => selectFunction(otherIdentifier, true));
    button.append(
      createElement("span", "detail__relation-name", otherIdentifier),
      createElement("span", "detail__relation-count", `${call.calls} call sites`),
    );
    item.append(button);
    const sites = createElement("details", "detail__evidence");
    sites.append(createElement("summary", "detail__evidence-summary", "Source locations"));
    const siteList = createElement("ul", "detail__site-list");
    for (const site of call.callSites) {
      siteList.append(
        createElement(
          "li",
          "detail__site",
          `${site.path}:${site.line}:${site.column}${call.testOnly ? " · test" : ""}`,
        ),
      );
    }
    sites.append(siteList);
    item.append(sites);
    container.append(item);
  }
}

function renderFunctionDetail(functionValue) {
  const component = componentByIdentifier(functionValue.component);
  elements.detailPanel.replaceChildren();
  const header = createElement("header", "detail__header");
  const heading = createElement("div", "detail__heading");
  const kind = createElement("span", "detail__kind", "Go function");
  kind.dataset.kind = component?.kind || "infrastructure";
  heading.append(kind, createElement("h2", "detail__title", functionValue.name));
  header.append(heading);
  const location = functionValue.path
    ? `${functionValue.id} · ${functionValue.path}:${functionValue.line}`
    : functionValue.id;
  elements.detailPanel.append(header, createElement("p", "detail__path", location));

  const metrics = createElement("div", "detail__metrics");
  appendMetric(metrics, functionValue.afferentCoupling, "afferent coupling (Cₐ)");
  appendMetric(metrics, functionValue.efferentCoupling, "efferent coupling (Cₑ)");
  appendMetric(metrics, formatRatio(functionValue.instability), "instability (I)");
  appendMetric(metrics, functionValue.incomingCallSites, "incoming call sites");
  appendMetric(metrics, functionValue.outgoingCallSites, "outgoing call sites");
  appendMetric(
    metrics,
    functionValue.crossComponentCallerFunctions,
    "cross-component caller functions",
  );
  appendMetric(
    metrics,
    functionValue.transitiveCallerFunctions,
    "transitive caller functions",
  );
  appendMetric(
    metrics,
    functionValue.transitiveCalledFunctions,
    "transitive called functions",
  );
  appendMetric(metrics, functionValue.testIncomingCallSites, "test incoming call sites");
  appendMetric(metrics, functionValue.usingApplicationCount, "applications that use this function");
  elements.detailPanel.append(metrics);

  const formula = createElement("div", "detail__formula");
  const totalCoupling = functionValue.afferentCoupling + functionValue.efferentCoupling;
  const instabilityFormula = totalCoupling === 0
    ? "Instability (I) = 0.00 · no resolved production coupling"
    : `Instability (I) = Cₑ / (Cₐ + Cₑ) = ${functionValue.efferentCoupling} / ` +
      `${totalCoupling} = ${formatRatio(functionValue.instability)}`;
  formula.append(
    createElement("code", "detail__formula-line", instabilityFormula),
    createElement(
      "code",
      "detail__formula-line",
      "Cₐ and Cₑ count unique production functions. Call-site counts retain repeated calls.",
    ),
    createElement(
      "code",
      "detail__formula-line",
      "Go static type information selects targets. Calls through function variables are not resolved.",
    ),
  );
  appendDetailSection(elements.detailPanel, "Function coupling calculation", formula);

  const visibleCalls = state.graph.functionCalls.filter(
    (call) => state.includeTests || !call.testOnly,
  );
  const callers = visibleCalls.filter((call) => call.target === functionValue.id);
  const callees = visibleCalls.filter((call) => call.source === functionValue.id);
  const callerList = createElement("ul", "detail__relation-list");
  appendFunctionCallList(callerList, callers, true);
  appendDetailSection(elements.detailPanel, "Caller functions", callerList);
  const calleeList = createElement("ul", "detail__relation-list");
  appendFunctionCallList(calleeList, callees, false);
  appendDetailSection(elements.detailPanel, "Called functions", calleeList);
}

function renderDetail() {
  const selectedFunction = functionByIdentifier(state.selectedFunctionIdentifier);
  if (state.viewMode === "functions" && selectedFunction) {
    renderFunctionDetail(selectedFunction);
    return;
  }
  const component = componentByIdentifier(state.selectedIdentifier);
  if (!component) {
    elements.detailPanel.replaceChildren();
    const empty = createElement("div", "detail__empty");
    const mark = createElement("span", "detail__empty-mark", "↗");
    mark.setAttribute("aria-hidden", "true");
    const functionView = state.viewMode === "functions";
    empty.append(
      mark,
      createElement(
        "h2",
        "detail__empty-title",
        functionView ? "Select a function" : "Select a component",
      ),
      createElement(
        "p",
        "detail__empty-copy",
        functionView
          ? "Review callers, called functions, and exact source locations."
          : "Review the imports and importers of the selected component.",
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
  appendMetric(metrics, component.afferentCoupling, "afferent coupling (Cₐ)");
  appendMetric(metrics, component.efferentCoupling, "efferent coupling (Cₑ)");
  appendMetric(metrics, formatRatio(component.instability), "instability (I)");
  appendMetric(metrics, formatRatio(component.abstractness), "abstractness (A)");
  appendMetric(metrics, formatRatio(component.mainSequenceDistance), "main-sequence distance (D)");
  appendMetric(
    metrics,
    `${component.abstractTypes}/${component.abstractTypes + component.concreteTypes}`,
    "abstract types",
  );
  appendMetric(metrics, component.usingApplicationCount, "applications that use this component");
  appendMetric(metrics, component.transitiveImportingComponents, "transitive importing components");
  appendMetric(metrics, component.sourceFiles, "Go files");
  elements.detailPanel.append(metrics);

  const stabilityFormula = createElement("div", "detail__formula");
  const totalCoupling = component.afferentCoupling + component.efferentCoupling;
  const instabilityFormula = totalCoupling === 0
    ? "Instability (I) = 0.00 · no external production coupling"
    : `Instability (I) = Cₑ / (Cₐ + Cₑ) = ${component.efferentCoupling} / ${totalCoupling} = ` +
      formatRatio(component.instability);
  const namedTypes = component.abstractTypes + component.concreteTypes;
  const abstractnessFormula = namedTypes === 0
    ? "Abstractness (A) = 0.00 · no named production types"
    : `Abstractness (A) = interface types / named types = ${component.abstractTypes} / ${namedTypes} = ` +
      formatRatio(component.abstractness);
  stabilityFormula.append(
    createElement("code", "detail__formula-line", instabilityFormula),
    createElement("code", "detail__formula-line", abstractnessFormula),
    createElement(
      "code",
      "detail__formula-line",
      `Main-sequence distance (D) = |A + I − 1| = ${formatRatio(component.mainSequenceDistance)}`,
    ),
    createElement(
      "code",
      "detail__formula-line",
      `Stable with low abstraction = Cₐ > 0, I ≤ 0.20, A ≤ 0.20 · ` +
        `${component.isStableWithLowAbstraction ? "yes" : "no"}`,
    ),
  );
  appendDetailSection(elements.detailPanel, "Stability calculation", stabilityFormula);

  if (component.usingApplications.length > 0) {
    const usingApplications = createElement("ul", "detail__application-list");
    for (const application of component.usingApplications) {
      usingApplications.append(createElement("li", "detail__application", application));
    }
    appendDetailSection(elements.detailPanel, "Applications that use this component", usingApplications);
  }

  const importingRelationships = state.graph.relationships
    .filter(
      (relationship) =>
        relationship.target === component.id && relationshipVisibleInDetail(relationship),
    )
    .sort((first, second) => first.source.localeCompare(second.source));
  const importingRelationshipList = createElement("ul", "detail__relation-list");
  if (importingRelationships.length === 0) {
    importingRelationshipList.append(
      createElement("li", "detail__none", "No component imports this component."),
    );
  } else {
    for (const relationship of importingRelationships) {
      const item = createElement("li", "detail__relation");
      item.append(relationshipButton(relationship, true));
      appendRelationshipEvidence(item, relationship);
      importingRelationshipList.append(item);
    }
  }
  appendDetailSection(elements.detailPanel, "Importing components", importingRelationshipList);

  const importedRelationships = state.graph.relationships
    .filter(
      (relationship) =>
        relationship.source === component.id && relationshipVisibleInDetail(relationship),
    )
    .sort((first, second) => first.target.localeCompare(second.target));
  const importedRelationshipList = createElement("ul", "detail__relation-list");
  if (importedRelationships.length === 0) {
    importedRelationshipList.append(
      createElement("li", "detail__none", "This component does not import another internal component."),
    );
  } else {
    for (const relationship of importedRelationships) {
      const item = createElement("li", "detail__relation");
      item.append(relationshipButton(relationship, false));
      appendRelationshipEvidence(item, relationship);
      importedRelationshipList.append(item);
    }
  }
  appendDetailSection(elements.detailPanel, "Imported components", importedRelationshipList);

  const componentFunctions = state.graph.functions
    .filter(
      (functionValue) =>
        functionValue.component === component.id && (state.includeTests || !functionValue.test),
    )
    .sort(
      (first, second) =>
        second.incomingCallSites - first.incomingCallSites || first.id.localeCompare(second.id),
    )
    .slice(0, 12);
  const functionList = createElement("ul", "detail__relation-list");
  if (componentFunctions.length === 0) {
    functionList.append(createElement("li", "detail__none", "No function is in the selected scope."));
  } else {
    for (const functionValue of componentFunctions) {
      const item = createElement("li", "detail__relation");
      const button = createElement("button", "detail__function-button");
      button.type = "button";
      button.title = functionValue.id;
      button.addEventListener("click", () => selectFunction(functionValue.id, true));
      button.append(
        createElement("span", "detail__relation-name", functionValue.name),
        createElement(
          "span",
          "detail__relation-count",
          `${functionValue.incomingCallSites} incoming calls · Cₐ ${functionValue.afferentCoupling}`,
        ),
      );
      item.append(button);
      functionList.append(item);
    }
  }
  appendDetailSection(elements.detailPanel, "Most called functions", functionList);

  const findingMessages = new Set();
  if (component.inCycle) {
    findingMessages.add("This component is in a production import cycle.");
  }
  if (component.isStableWithLowAbstraction) {
    findingMessages.add(
      "One or more production components import this stable component, which has low abstraction.",
    );
  }
  for (const relationship of [...importingRelationships, ...importedRelationships]) {
    for (const ruleViolation of relationship.ruleViolations) {
      findingMessages.add(ruleViolationLabels[ruleViolation] || ruleViolation);
    }
  }
  if (findingMessages.size > 0) {
    const findingList = createElement("ul", "detail__finding-list");
    for (const findingMessage of [...findingMessages].sort()) {
      findingList.append(createElement("li", "detail__finding", findingMessage));
    }
    appendDetailSection(elements.detailPanel, "Findings", findingList);
  }
}

function selectComponent(identifier, makeVisible) {
  const component = componentByIdentifier(identifier);
  if (!component) {
    return;
  }
  let filtersChanged = false;
  if (makeVisible && !state.visibleKinds.has(component.kind)) {
    state.visibleKinds.add(component.kind);
    filtersChanged = true;
  }
  if (makeVisible && state.search) {
    state.search = "";
    elements.searchInput.value = "";
    filtersChanged = true;
  }
  state.selectedIdentifier = identifier;
  state.selectedFunctionIdentifier = "";
  if (state.viewMode === "functions") {
    state.viewMode = "metrics";
  }
  if (filtersChanged) {
    renderKindControls();
  }
  renderActiveMap(filtersChanged);
  renderDetail();
}

function selectFunction(identifier, makeVisible) {
  const functionValue = functionByIdentifier(identifier);
  if (!functionValue) {
    return;
  }
  const component = componentByIdentifier(functionValue.component);
  let filtersChanged = false;
  if (makeVisible && component && !state.visibleKinds.has(component.kind)) {
    state.visibleKinds.add(component.kind);
    filtersChanged = true;
  }
  if (makeVisible && state.search) {
    state.search = "";
    elements.searchInput.value = "";
    filtersChanged = true;
  }
  state.selectedFunctionIdentifier = identifier;
  state.selectedIdentifier = functionValue.component;
  state.viewMode = "functions";
  if (filtersChanged) {
    renderKindControls();
  }
  renderActiveMap(false);
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

function appendFunctionRankingEntry(list, functionValue, position) {
  const component = componentByIdentifier(functionValue.component);
  const item = createElement("li", "ranking__item");
  const button = createElement("button", "ranking__button");
  button.type = "button";
  button.dataset.kind = component?.kind || "infrastructure";
  button.title = functionValue.id;
  button.addEventListener("click", () => selectFunction(functionValue.id, true));
  const incomingCalls = functionValue.incomingCallSites +
    (state.includeTests ? functionValue.testIncomingCallSites : 0);
  button.append(
    createElement("span", "ranking__position", position),
    createElement("span", "ranking__name", functionValue.name),
    createElement("span", "ranking__value", `${incomingCalls} calls · Cₐ ${functionValue.afferentCoupling}`),
  );
  item.append(button);
  list.append(item);
}

function renderRankings() {
  elements.usageRanking.replaceChildren();
  const importedComponentKinds = new Set([
    "application-module",
    "shared-module",
    "library",
    "infrastructure",
  ]);
  const componentsWithProductionImporters = state.graph.components
    .filter(
      (component) =>
        importedComponentKinds.has(component.kind) && component.productionImportingComponents > 0,
    )
    .sort(compareByImportingComponents)
    .slice(0, 7);
  if (componentsWithProductionImporters.length === 0) {
    elements.usageRanking.append(createElement("li", "detail__none", "No production relationship exists."));
  } else {
    componentsWithProductionImporters.forEach((component, index) => {
      appendRankingEntry(
        elements.usageRanking,
        component,
        String(index + 1).padStart(2, "0"),
        `${component.productionImportingComponents} importing components`,
      );
    });
  }

  elements.functionUsageRanking.replaceChildren();
  const calledFunctions = state.graph.functions
    .filter(
      (functionValue) =>
        (state.includeTests || !functionValue.test) &&
        functionValue.incomingCallSites +
          (state.includeTests ? functionValue.testIncomingCallSites : 0) > 0,
    )
    .sort(
      (first, second) =>
        second.incomingCallSites +
          (state.includeTests ? second.testIncomingCallSites : 0) -
          (first.incomingCallSites +
            (state.includeTests ? first.testIncomingCallSites : 0)) ||
        second.afferentCoupling - first.afferentCoupling ||
        first.id.localeCompare(second.id),
    )
    .slice(0, 10);
  if (calledFunctions.length === 0) {
    elements.functionUsageRanking.append(
      createElement("li", "detail__none", "No resolved internal function call exists."),
    );
  } else {
    calledFunctions.forEach((functionValue, index) => {
      appendFunctionRankingEntry(
        elements.functionUsageRanking,
        functionValue,
        String(index + 1).padStart(2, "0"),
      );
    });
  }

  elements.unusedRanking.replaceChildren();
  const componentKindsThatCanHaveImporters = new Set([
    "application-module",
    "shared-module",
    "library",
  ]);
  const componentsWithoutProductionImporters = state.graph.components
    .filter(
      (component) =>
        componentKindsThatCanHaveImporters.has(component.kind) &&
        component.productionImportingComponents === 0,
    )
    .sort(compareIdentifiers)
    .slice(0, 7);
  if (componentsWithoutProductionImporters.length === 0) {
    elements.unusedRanking.append(
      createElement("li", "detail__none", "Each listed component has a production importer."),
    );
  } else {
    componentsWithoutProductionImporters.forEach((component, index) => {
      appendRankingEntry(
        elements.unusedRanking,
        component,
        String(index + 1).padStart(2, "0"),
        `${component.sourceFiles} files`,
      );
    });
  }

  elements.stableLowAbstractionRanking.replaceChildren();
  const stableLowAbstractionComponents = state.graph.components
    .filter((component) => component.isStableWithLowAbstraction)
    .sort(
      (first, second) =>
        second.afferentCoupling - first.afferentCoupling ||
        second.mainSequenceDistance - first.mainSequenceDistance ||
        compareIdentifiers(first, second),
    )
    .slice(0, 7);
  if (stableLowAbstractionComponents.length === 0) {
    elements.stableLowAbstractionRanking.append(
      createElement("li", "detail__none", "No stable component has low abstraction."),
    );
  } else {
    stableLowAbstractionComponents.forEach((component, index) => {
      appendRankingEntry(
        elements.stableLowAbstractionRanking,
        component,
        String(index + 1).padStart(2, "0"),
        `afferent coupling ${component.afferentCoupling} · ` +
          `instability ${formatRatio(component.instability)}`,
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
    `The analyzer finds errors in ${diagnostics.length} source files`;
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
          throw new Error(`The server returns status ${response.status}.`);
        }
        const graph = await response.json();
        if (state.graph?.revision !== graph.revision) {
          state.graph = graph;
          if (state.selectedIdentifier && !componentByIdentifier(state.selectedIdentifier)) {
            state.selectedIdentifier = "";
          }
          if (
            state.selectedFunctionIdentifier &&
            !functionByIdentifier(state.selectedFunctionIdentifier)
          ) {
            state.selectedFunctionIdentifier = "";
          }
          renderDashboard();
        }
      } while (state.graphRequestPending);
      return true;
    } catch (error) {
      setConnectionState("error", "Cannot load the dependency graph");
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
    setConnectionState("live", "The connection is active.");
    if (state.graph?.revision !== event.data) {
      requestGraph();
    }
  });
  events.addEventListener("graph", () => {
    state.eventStreamConnected = true;
    setConnectionState("live", "The client loads the updated graph.");
    requestGraph().then((succeeded) => {
      if (succeeded && state.eventStreamConnected) {
        setConnectionState("live", "The connection is active.");
      }
    });
  });
  events.onerror = () => {
    state.eventStreamConnected = false;
    setConnectionState("connecting", "The client reconnects to the server.");
  };
}

elements.searchInput.addEventListener("input", () => {
  state.search = normalizeSearch(elements.searchInput.value);
  renderActiveMap(true);
});

elements.testToggle.addEventListener("change", () => {
  state.includeTests = elements.testToggle.checked;
  renderRankings();
  renderActiveMap(false);
  renderDetail();
});

elements.componentMetricView.addEventListener("click", () => setViewMode("metrics"));
elements.functionMetricView.addEventListener("click", () => setViewMode("functions"));
elements.dependencyView.addEventListener("click", () => setViewMode("dependencies"));
elements.lightTheme.addEventListener("click", () => setTheme("light", true));
elements.darkTheme.addEventListener("click", () => setTheme("dark", true));

elements.componentMetric.addEventListener("change", () => {
  if (!componentMetricDefinitions[elements.componentMetric.value]) {
    return;
  }
  state.componentMetric = elements.componentMetric.value;
  renderActiveMap(false);
});

elements.functionMetric.addEventListener("change", () => {
  if (!functionMetricDefinitions[elements.functionMetric.value]) {
    return;
  }
  state.functionMetric = elements.functionMetric.value;
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

elements.showAllComponents.addEventListener("click", showAllComponents);

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
    if (state.viewMode === "metrics") {
      renderComponentMetricMap();
    } else if (state.viewMode === "functions") {
      renderFunctionMetricMap();
    } else {
      applyTransform();
    }
  }
}).observe(elements.graphViewport);

setTheme(storedTheme(), false);
setConnectionState("connecting", "The client connects.");
requestGraph();
connectEventStream();
