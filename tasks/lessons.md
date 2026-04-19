# Naturalize — Lessons

## 2026-04-19

- When the user reports that a round "completed" but produced no usable output, verify the actual provider endpoint and compare real `input/output` samples before changing UI behavior.
- If the same symptom survives a UI-side patch, treat it as a new backend/runtime investigation and re-check the live dependency graph, especially local mock services and port mappings.
- For any workflow the user expects to survive refresh or re-entry, do not rely on frontend memory or SSE alone; persist recoverable progress and activity timeline on the backend, then make the frontend restore from a single server-side state endpoint.
- If the UI labels a field as `模型`, never feed it a provider alias like `openai-responses`; trace the value back to the LLM client and verify the runtime payload carries the configured model name.
