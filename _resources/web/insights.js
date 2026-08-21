"use strict";

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
  const relatedComponentIdentifier = sourceImportsSelectedComponent
    ? relationship.source
    : relationship.target;
  const relatedComponent = componentByIdentifier(relatedComponentIdentifier);
  const button = createElement("button", "detail__relation-button");
  button.type = "button";
  button.title = relationship.targetPackages.join(", ");
  button.addEventListener("click", () => selectComponent(relatedComponentIdentifier, true));
  const name = createElement(
    "span",
    "detail__relation-name",
    relatedComponent?.name || relatedComponentIdentifier,
  );
  const referencingFiles =
    relationship.productionReferencingFiles + relationship.testReferencingFiles;
  const suffix = relationship.testOnly
    ? " · test only"
    : relationship.ruleViolations.length > 0
      ? ` · ${relationship.ruleViolations.length} architecture ` +
        (relationship.ruleViolations.length === 1 ? "finding" : "findings")
      : "";
  const functionCallSites =
    relationship.productionFunctionCallSites + relationship.testFunctionCallSites;
  const callSiteText = functionCallSites > 0 ? ` · ${functionCallSites} call sites` : "";
  const count = createElement(
    "span",
    "detail__relation-count",
    `${referencingFiles} files${callSiteText}${suffix}`,
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
    evidenceList.append(
      createElement("li", "detail__none", "No visible call site or import declaration exists."),
    );
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
        incoming
          ? "No visible caller function calls this function."
          : "This function calls no visible callee function.",
      ),
    );
    return;
  }
  for (const call of calls) {
    const relatedFunctionIdentifier = incoming ? call.source : call.target;
    const item = createElement("li", "detail__relation");
    const button = createElement("button", "detail__function-button");
    button.type = "button";
    button.title = relatedFunctionIdentifier;
    button.addEventListener("click", () => selectFunction(relatedFunctionIdentifier, true));
    button.append(
      createElement("span", "detail__relation-name", relatedFunctionIdentifier),
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
  const categoryBadge = createElement("span", "detail__category", "Go function");
  setCategoryPresentation(categoryBadge, component);
  heading.append(categoryBadge, createElement("h2", "detail__title", functionValue.name));
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
    functionValue.transitiveCalleeFunctions,
    "transitive callee functions",
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
      "The analyzer resolves call targets with Go static type information. " +
        "The analyzer does not resolve calls through function variables.",
    ),
  );
  appendDetailSection(elements.detailPanel, "Function coupling calculation", formula);

  const visibleCalls = state.graph.functionCalls.filter(
    (call) => state.includeTests || !call.testOnly,
  );
  const incomingFunctionCalls = visibleCalls.filter((call) => call.target === functionValue.id);
  const outgoingFunctionCalls = visibleCalls.filter((call) => call.source === functionValue.id);
  const callerList = createElement("ul", "detail__relation-list");
  appendFunctionCallList(callerList, incomingFunctionCalls, true);
  appendDetailSection(elements.detailPanel, "Caller functions", callerList);
  const calleeList = createElement("ul", "detail__relation-list");
  appendFunctionCallList(calleeList, outgoingFunctionCalls, false);
  appendDetailSection(elements.detailPanel, "Callee functions", calleeList);
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
          ? "Review caller functions, callee functions, and exact source locations."
          : "Review the imports and importers of the selected component.",
      ),
    );
    elements.detailPanel.append(empty);
    return;
  }

  elements.detailPanel.replaceChildren();
  const header = createElement("header", "detail__header");
  const heading = createElement("div", "detail__heading");
  const categoryBadge = createElement(
    "span",
    "detail__category",
    componentClassificationLabel(component),
  );
  setCategoryPresentation(categoryBadge, component);
  heading.append(categoryBadge, createElement("h2", "detail__title", component.name));
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
    functionList.append(
      createElement("li", "detail__none", "No visible function belongs to this component."),
    );
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
          `${functionValue.incomingCallSites} incoming call sites · ` +
            `Cₐ ${functionValue.afferentCoupling}`,
        ),
      );
      item.append(button);
      functionList.append(item);
    }
  }
  appendDetailSection(
    elements.detailPanel,
    "Functions with the most incoming call sites",
    functionList,
  );

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
      findingMessages.add(relationshipFindingMessage(ruleViolation, relationship));
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
  const category = componentCategory(component);
  if (makeVisible && !state.visibleCategories.has(category)) {
    state.visibleCategories.add(category);
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
    renderCategoryControls();
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
  const category = componentCategory(component);
  if (makeVisible && component && !state.visibleCategories.has(category)) {
    state.visibleCategories.add(category);
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
    renderCategoryControls();
  }
  renderActiveMap(false);
  renderDetail();
}

function appendRankingEntry(list, component, position, value) {
  const item = createElement("li", "ranking__item");
  const button = createElement("button", "ranking__button");
  button.type = "button";
  setCategoryPresentation(button, component);
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
  setCategoryPresentation(button, component);
  button.title = functionValue.id;
  button.addEventListener("click", () => selectFunction(functionValue.id, true));
  const incomingCallSites = functionValue.incomingCallSites +
    (state.includeTests ? functionValue.testIncomingCallSites : 0);
  button.append(
    createElement("span", "ranking__position", position),
    createElement("span", "ranking__name", functionValue.name),
    createElement(
      "span",
      "ranking__value",
      `${incomingCallSites} call sites · Cₐ ${functionValue.afferentCoupling}`,
    ),
  );
  item.append(button);
  list.append(item);
}

function renderRankings() {
  elements.usageRanking.replaceChildren();
  const importedComponentRoles = new Set([
    "application-module",
    "shared-module",
    "library",
    "infrastructure",
  ]);
  const componentsWithProductionImporters = state.graph.components
    .filter(
      (component) =>
        importedComponentRoles.has(component.role) && component.productionImportingComponents > 0,
    )
    .sort(compareByImportingComponents)
    .slice(0, 7);
  if (componentsWithProductionImporters.length === 0) {
    elements.usageRanking.append(
      createElement("li", "detail__none", "No listed component has a production importer."),
    );
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
  const functionsWithIncomingCallSites = state.graph.functions
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
  if (functionsWithIncomingCallSites.length === 0) {
    elements.functionUsageRanking.append(
      createElement("li", "detail__none", "No visible resolved internal function call exists."),
    );
  } else {
    functionsWithIncomingCallSites.forEach((functionValue, index) => {
      appendFunctionRankingEntry(
        elements.functionUsageRanking,
        functionValue,
        String(index + 1).padStart(2, "0"),
      );
    });
  }

  elements.componentsWithoutProductionImportersRanking.replaceChildren();
  const componentRolesThatCanHaveImporters = new Set([
    "application-module",
    "shared-module",
    "library",
  ]);
  const componentsWithoutProductionImporters = state.graph.components
    .filter(
      (component) =>
        componentRolesThatCanHaveImporters.has(component.role) &&
        component.productionImportingComponents === 0,
    )
    .sort(compareIdentifiers)
    .slice(0, 7);
  if (componentsWithoutProductionImporters.length === 0) {
    elements.componentsWithoutProductionImportersRanking.append(
      createElement("li", "detail__none", "Each listed component has a production importer."),
    );
  } else {
    componentsWithoutProductionImporters.forEach((component, index) => {
      appendRankingEntry(
        elements.componentsWithoutProductionImportersRanking,
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
    `The analyzer finds ${diagnostics.length} source diagnostics`;
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
