# Naturalize — Lessons

## 2026-04-19

- When the user reports that a round "completed" but produced no usable output, verify the actual provider endpoint and compare real `input/output` samples before changing UI behavior.
- If the same symptom survives a UI-side patch, treat it as a new backend/runtime investigation and re-check the live dependency graph, especially local mock services and port mappings.
