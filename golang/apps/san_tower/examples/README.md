# san_tower examples

Workflows ready to import into a tower (Workflows page › Import, or
WorkflowService's ImportWorkflows), each made in a tower and exported with
ExportWorkflows, so the files are exactly the tower's format (`WorkflowFile`
JSON, format `san_tower.workflows`, version 1). Connections and storages are
named, never their settings: the import picks this tower's of the same name
and type, and its notes say what is left to pick. Imported workflows are
switched off.

- [openai-api](openai-api/README.md): the tower as an OpenAI-compatible
  chat API answered by ChatGPT, Gemini or Claude in the browser: stateless
  (a new Temporary chat per request), in a kept chat (Browser Chat Session,
  only the newest message per request), and a router whose `model` picks
  the site.

The trading workflows, with their setup script, are in
[../workflows/trading](../workflows/trading/README.md).
