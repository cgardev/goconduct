"use strict";

const svgNamespace = "http://www.w3.org/2000/svg";
const numberFormatter = new Intl.NumberFormat("en-GB");
const themeStorageKey = "dependencygraph-theme";
const supportedThemes = new Set(["light", "dark"]);

const knownCategoryNames = Object.freeze({
  application: { label: "Application", plural: "Applications" },
  "application-module": { label: "Application module", plural: "Application modules" },
  "shared-module": { label: "Shared module", plural: "Shared modules" },
  library: { label: "Library", plural: "Libraries" },
  infrastructure: { label: "Infrastructure", plural: "Infrastructure" },
  development: { label: "Development tool", plural: "Development tools" },
});

const categoryPaletteSize = 12;
const knownCategoryPaletteIndexes = Object.freeze({
  application: 0,
  "application-module": 1,
  "shared-module": 2,
  library: 3,
  infrastructure: 4,
  development: 5,
});

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
  incomingCallSites: {
    label: "incoming call sites",
    shortLabel: "incoming call sites",
  },
  afferent: {
    label: "caller functions, or afferent coupling (Cₐ)",
    shortLabel: "Cₐ",
  },
  efferent: {
    label: "callee functions, or efferent coupling (Cₑ)",
    shortLabel: "Cₑ",
  },
  outgoingCallSites: {
    label: "outgoing call sites",
    shortLabel: "outgoing call sites",
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
  categoryGap: 58,
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
  summaryCategories: document.getElementById("summaryCategories"),
  summaryRelationships: document.getElementById("summaryRelationships"),
  summaryTests: document.getElementById("summaryTests"),
  summaryFunctions: document.getElementById("summaryFunctions"),
  summaryFunctionMetrics: document.getElementById("summaryFunctionMetrics"),
  summaryUsingApplications: document.getElementById("summaryUsingApplications"),
  summaryUsingApplicationsNote: document.getElementById("summaryUsingApplicationsNote"),
  summaryFindings: document.getElementById("summaryFindings"),
  summaryFindingBreakdown: document.getElementById("summaryFindingBreakdown"),
  searchInput: document.getElementById("searchInput"),
  categoryFilters: document.getElementById("categoryFilters"),
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
  fitVisibleComponents: document.getElementById("fitVisibleComponents"),
  zoomIn: document.getElementById("zoomIn"),
  detailPanel: document.getElementById("detailPanel"),
  usageRanking: document.getElementById("usageRanking"),
  functionUsageRanking: document.getElementById("functionUsageRanking"),
  componentsWithoutProductionImportersRanking: document.getElementById(
    "componentsWithoutProductionImportersRanking",
  ),
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
  visibleCategories: new Set(),
  includeTests: true,
  search: "",
  viewMode: "metrics",
  componentMetric: "importingComponents",
  functionMetric: "incomingCallSites",
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

function componentCategory(component) {
  return component?.category || String(component?.role || "infrastructure");
}

function categoryName(identifier) {
  return identifier
    .split(/[-_/]+/u)
    .filter(Boolean)
    .join(" ");
}

function categoryDefinition(identifier) {
  const knownName = knownCategoryNames[identifier];
  if (knownName) {
    return { identifier, ...knownName };
  }
  const name = categoryName(identifier);
  const label = name ? name.charAt(0).toLocaleUpperCase("en-GB") + name.slice(1) : "Component";
  return { identifier, label, plural: `${label} components` };
}

function componentClassificationLabel(component) {
  const category = componentCategory(component);
  const categoryLabel = categoryDefinition(category).label;
  const role = String(component?.role || "infrastructure");
  if (category === role) {
    return categoryLabel;
  }
  return `${categoryLabel} · ${categoryDefinition(role).label} role`;
}

function graphCategoryDefinitions() {
  const categories = new Set(
    (state.graph?.components || []).map((component) => componentCategory(component)),
  );
  return [...categories].sort().map(categoryDefinition);
}

function categoryPaletteIndex(identifier) {
  if (knownCategoryPaletteIndexes[identifier] !== undefined) {
    return knownCategoryPaletteIndexes[identifier];
  }
  let value = 0;
  [...identifier].forEach((character, index) => {
    value = (value + character.codePointAt(0) * (index + 1)) % categoryPaletteSize;
  });
  return value;
}

function setCategoryPresentation(element, component) {
  const category = componentCategory(component);
  element.dataset.category = category;
  element.dataset.colorIndex = String(categoryPaletteIndex(category));
}

function addNewGraphCategories(graph, previousGraph) {
  const previousCategories = new Set(
    (previousGraph?.components || []).map((component) => componentCategory(component)),
  );
  for (const component of graph.components || []) {
    const category = componentCategory(component);
    if (!previousGraph || !previousCategories.has(category)) {
      state.visibleCategories.add(category);
    }
  }
}

function relationshipFindingMessage(rule, relationship) {
  const finding = (state.graph?.findings || []).find(
    (candidate) =>
      candidate.rule === rule &&
      candidate.source === relationship.source &&
      candidate.target === relationship.target,
  );
  return finding?.message || rule;
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
    componentCategory(component),
    component.role,
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
    (component) =>
      state.visibleCategories.has(componentCategory(component)) && componentMatchesSearch(component),
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
      state.visibleCategories.has(componentCategory(component)) &&
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
  return functionMetricDefinitions[state.functionMetric] || functionMetricDefinitions.incomingCallSites;
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
    case "outgoingCallSites":
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
