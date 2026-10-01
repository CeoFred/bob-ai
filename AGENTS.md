# Agent Instructions & Guidelines

## Knowledge Graph Maintenance (Graphify)

- **Always Update Graphify at the End of Each Task**:
  At the completion of any task that adds, modifies, or deletes code, configuration, or documentation files in the repository, the agent **must** update the graphify knowledge graph using:
  ```bash
  $(cat graphify-out/.graphify_python) -m graphify --update
  ```
  *(or invoke `/graphify --update`)* to ensure `graphify-out/graph.json`, `graphify-out/GRAPH_REPORT.md`, and visualizations stay synchronized with the latest codebase state.

- **Token Efficiency & Codebase Queries**:
  When answering questions about codebase architecture, dependencies, data flow, or cross-module relationships, prioritize querying the local graph (`graphify-out/graph.json`) via `graphify query "<question>"` for maximum token efficiency before reading individual files.
