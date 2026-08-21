"use strict";

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
  setCategoryPresentation(node, component);
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
    const categoryLabel = createSvgElement("text", "component-metric-map__tile-category");
    categoryLabel.setAttribute("x", String(inset + 10));
    categoryLabel.setAttribute("y", String(height - inset - 10));
    categoryLabel.textContent = categoryDefinition(componentCategory(component)).label;
    node.append(categoryLabel);
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
  setCategoryPresentation(node, component);
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
    const componentName = createSvgElement("text", "function-metric-map__tile-component");
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

function renderCategoryControls() {
  elements.categoryFilters.replaceChildren();
  elements.legend.replaceChildren();
  const definitions = graphCategoryDefinitions();
  const counts = new Map(definitions.map((definition) => [definition.identifier, 0]));
  for (const component of state.graph?.components || []) {
    const category = componentCategory(component);
    counts.set(category, (counts.get(category) || 0) + 1);
  }

  for (const definition of definitions) {
    const button = createElement("button", "toolbar__category");
    button.type = "button";
    setCategoryPresentation(button, { category: definition.identifier });
    button.setAttribute(
      "aria-pressed",
      String(state.visibleCategories.has(definition.identifier)),
    );
    button.setAttribute("aria-label", `Show or hide ${definition.plural.toLocaleLowerCase("en-GB")}`);

    const dot = createElement("span", "toolbar__category-dot");
    dot.setAttribute("aria-hidden", "true");
    const label = createElement("span", "toolbar__category-label", definition.label);
    const count = createElement(
      "span",
      "toolbar__category-count",
      numberFormatter.format(counts.get(definition.identifier) || 0),
    );
    button.append(dot, label, count);
    button.addEventListener("click", () => {
      if (state.visibleCategories.has(definition.identifier)) {
        state.visibleCategories.delete(definition.identifier);
      } else {
        state.visibleCategories.add(definition.identifier);
      }
      button.setAttribute(
        "aria-pressed",
        String(state.visibleCategories.has(definition.identifier)),
      );
      renderActiveMap(true);
    });
    elements.categoryFilters.append(button);

    const legendItem = createElement("span", "legend__item");
    setCategoryPresentation(legendItem, { category: definition.identifier });
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
  const categorySummary = Object.entries(summary.categories || {})
    .sort(([first], [second]) => first.localeCompare(second))
    .map(([identifier, count]) => {
      const definition = categoryDefinition(identifier);
      const label = count === 1 ? definition.label : definition.plural;
      return `${numberFormatter.format(count)} ${label.toLocaleLowerCase("en-GB")}`;
    });
  elements.summaryCategories.textContent = categorySummary.length > 0
    ? categorySummary.join(" · ")
    : `${summary.applications} applications · ` +
      `${summary.sharedModules + summary.applicationModules} modules · ` +
      `${summary.libraries} libraries`;
  elements.summaryRelationships.textContent = numberFormatter.format(summary.productionRelationships);
  elements.summaryTests.textContent = `${summary.testOnlyRelationships} test-only relationships`;
  elements.summaryFunctions.textContent = numberFormatter.format(summary.productionFunctions);
  elements.summaryFunctionMetrics.textContent =
    `${numberFormatter.format(summary.functionCallSites)} resolved call sites · ` +
    `${numberFormatter.format(summary.testFunctions)} test functions · ` +
    `${numberFormatter.format(summary.functionCycles)} call cycles`;

  const sharedComponents = components
    .filter((component) => !["application", "development"].includes(component.role))
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
  elements.summaryFindingBreakdown.textContent =
    `${summary.errors} errors · ${summary.warnings} warnings · ` +
    `${summary.stableDependencyPrincipleViolations} stable dependency principle violations`;
  elements.revisionLabel.textContent = `graph revision ${state.graph.revision.slice(0, 12)}`;
}

function layoutComponents(components) {
  const positions = new Map();
  const groups = [];
  let currentX = layoutOptions.left;
  let maximumRows = 0;

  for (const definition of graphCategoryDefinitions()) {
    const members = components
      .filter((component) => componentCategory(component) === definition.identifier)
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
    currentX += columnCount * layoutOptions.columnStep + layoutOptions.categoryGap;
  }

  return {
    positions,
    groups,
    width: Math.max(
      currentX - layoutOptions.categoryGap + layoutOptions.right,
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
    setCategoryPresentation(label, { category: group.definition.identifier });
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
    const referencingFiles =
      relationship.productionReferencingFiles + relationship.testReferencingFiles;
    const sourceComponent = componentByIdentifier(relationship.source);
    const targetComponent = componentByIdentifier(relationship.target);
    const stability = sourceComponent && targetComponent
      ? `\nInstability (I): ${formatRatio(sourceComponent.instability)} → ` +
        `${formatRatio(targetComponent.instability)}`
      : "";
    const violations = relationship.ruleViolations
      .map((rule) => relationshipFindingMessage(rule, relationship))
      .join("\n");
    const violationText = violations ? `\n${violations}` : "";
    title.textContent =
      `${relationship.source} → ${relationship.target}\n` +
      `${referencingFiles} files · ${relationship.sourcePackages.length} source packages` +
      `${stability}${violationText}`;
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
    setCategoryPresentation(node, component);
    node.setAttribute("transform", `translate(${position.x} ${position.y})`);
    node.setAttribute("tabindex", "0");
    node.setAttribute("role", "button");
    node.setAttribute(
      "aria-label",
      `${component.name}, ${categoryDefinition(componentCategory(component)).label}`,
    );

    const box = createSvgElement("rect", "dependency-map__node-box");
    box.setAttribute("width", String(layoutOptions.nodeWidth));
    box.setAttribute("height", String(layoutOptions.nodeHeight));
    box.setAttribute("rx", "8");

    const categoryDot = createSvgElement("circle", "dependency-map__node-category");
    categoryDot.setAttribute("cx", "14");
    categoryDot.setAttribute("cy", "15");
    categoryDot.setAttribute("r", "4");

    const name = createSvgElement("text", "dependency-map__node-name");
    name.setAttribute("x", "24");
    name.setAttribute("y", "18");
    name.textContent = shortenedName(component.name);

    const metric = createSvgElement("text", "dependency-map__node-metric");
    metric.setAttribute("x", "14");
    metric.setAttribute("y", "35");
    metric.textContent =
      `Afferent coupling ${component.afferentCoupling} · ` +
      `efferent coupling ${component.efferentCoupling} · ` +
      `instability ${formatRatio(component.instability)}`;

    const title = createSvgElement("title");
    title.textContent =
      `${component.id}\nAfferent coupling (Cₐ): ${component.afferentCoupling} · ` +
      `Efferent coupling (Cₑ): ${component.efferentCoupling}\n` +
      `Instability (I): ${formatRatio(component.instability)} · ` +
      `Abstractness (A): ${formatRatio(component.abstractness)} · ` +
      `Main-sequence distance (D): ${formatRatio(component.mainSequenceDistance)}`;
    node.append(box, categoryDot, name, metric, title);

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

function renderGraph(shouldFitVisibleComponents) {
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
  if (shouldFitVisibleComponents) {
    fitVisibleComponents();
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
      `The tile area represents this component metric: ${selectedMetric.label}. ` +
      "A larger tile represents a higher metric value.";
    elements.componentMetricLegendText.textContent =
      `A larger area represents a higher value for ${selectedMetric.label}.`;
    elements.graphViewport.setAttribute(
      "aria-label",
      "Component metric map. A larger tile represents a higher metric value.",
    );
    return;
  }

  if (functionMapVisible) {
    const selectedMetric = selectedFunctionMetric();
    elements.graphEyebrow.textContent = "Resolved static function calls";
    elements.graphTitle.textContent = "Function usage map";
    elements.graphDescription.textContent =
      `The tile area represents ${selectedMetric.label}. ` +
      "The analyzer resolves each function call target with Go static type information.";
    elements.componentMetricLegendText.textContent =
      `A larger area represents a higher value for ${selectedMetric.label}.`;
    elements.graphViewport.setAttribute(
      "aria-label",
      "Function metric map. A larger tile represents a higher metric value.",
    );
    return;
  }

  elements.graphEyebrow.textContent = "Import relationships";
  elements.graphTitle.textContent = "Component import map";
  elements.graphDescription.textContent =
    "Arrows point from an importing component to the imported internal component.";
  elements.graphViewport.setAttribute(
    "aria-label",
    "Dependency graph. Drag the map to move it. Use the wheel to zoom.",
  );
}

function renderActiveMap(shouldFitVisibleComponents) {
  renderViewState();
  if (state.viewMode === "metrics") {
    renderComponentMetricMap();
    return;
  }
  if (state.viewMode === "functions") {
    renderFunctionMetricMap();
    return;
  }
  renderGraph(shouldFitVisibleComponents);
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

function fitVisibleComponents() {
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
