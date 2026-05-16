# Baseline Evaluation Report (2026-05-16, Full-100)

- Dataset: /Users/iluwen/Documents/Code/Workspace/Go/AI_Agents/Naturalize/testdata/eval/dataset.100.jsonl
- Scope: full dataset (`--max-samples 100`)
- Base URL: http://127.0.0.1:8080
- Completed samples: 95 / 100
- Failed samples: 5
- Avg score before: 0.457364
- Avg score after: 0.458280
- Avg delta (before-after): -0.000916
- Raw metrics: metrics-20260516-142627.json

## Source Breakdown

- AI source: count=65, before=0.488869, after=0.490207, delta=-0.001338
- Human source: count=30, before=0.389104, after=0.389104, delta=0.000000

## Failed Samples

- cn-ai-008
- cn-ai-012
- cn-ai-020
- cn-ai-024
- cn-ai-032

## Notes

- Current run still uses `internal-calibrated` detector path (no external detector configured).
- Negative delta indicates average score increased slightly after rewrite in this baseline.
