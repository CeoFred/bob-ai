# Knowledge Graph & Token Efficiency Rules

- **End-of-Task Knowledge Graph Synchronization**:
  At the end of every task where code, documentation, scripts, or configurations are created, modified, or deleted, run the graphify update command:
  ```bash
  $(cat graphify-out/.graphify_python) -m graphify --update
  ```
  *(or invoke `/graphify --update`)* to keep `graphify-out/graph.json`, `graphify-out/GRAPH_REPORT.md`, and visualizations synchronized.

- **Query-First Architecture Exploration**:
  When investigating architecture, cross-package relationships, or code dependencies, query the persistent knowledge graph (`graphify-out/graph.json`) via `graphify query "<question>"` to minimize token consumption before reading individual source files.
