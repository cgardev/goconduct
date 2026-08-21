"use strict";

function renderDashboard() {
  renderSummary();
  renderCategoryControls();
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
          addNewGraphCategories(graph, state.graph);
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
      setConnectionState("error", "The client cannot load the dependency graph.");
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
  const eventStream = new EventSource("/api/events");
  eventStream.addEventListener("ready", (event) => {
    state.eventStreamConnected = true;
    setConnectionState("live", "The connection is active.");
    if (state.graph?.revision !== event.data) {
      requestGraph();
    }
  });
  eventStream.addEventListener("graph", () => {
    state.eventStreamConnected = true;
    setConnectionState("live", "The client loads the updated graph.");
    requestGraph().then((succeeded) => {
      if (succeeded && state.eventStreamConnected) {
        setConnectionState("live", "The connection is active.");
      }
    });
  });
  eventStream.onerror = () => {
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

elements.fitVisibleComponents.addEventListener("click", fitVisibleComponents);

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
