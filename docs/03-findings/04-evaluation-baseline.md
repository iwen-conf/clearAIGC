# P0 评测基线落地说明

本文档对应 `03-gap-analysis-and-benchmarks.md` 里的 P0 动作，目标是把“降 AI 风险”的结论改成可回归、可量化的数字。

## 新增内容

- 数据集：`testdata/eval/dataset.100.jsonl`
  - 100 条样本（中英混合，`ai/human` 来源字段，`academic/technical/spoken` 领域字段）
- 数据集生成脚本：`scripts/generate_eval_dataset.sh`
- 评测脚本：`scripts/evaluate.sh`
- 快捷入口：`scripts/smoke_mock_e2e.sh --evaluate`
- 报告目录：`var/data/reports/`

## 数据格式（JSONL）

每行一个样本：

```json
{"id":"cn-ai-001","profile":"cn","source":"ai","domain":"academic","text":"..."}
```

字段说明：

- `id`: 样本 ID
- `profile`: `cn | cn_single | en`
- `source`: `ai | human`
- `domain`: `academic | technical | spoken`
- `text`: 输入正文

## 运行方式

### 1) 默认跑 100 条（本地 mock）

```bash
bash scripts/evaluate.sh
```

### 2) 用 smoke 脚本入口跑评测

```bash
bash scripts/smoke_mock_e2e.sh --evaluate
```

### 3) 只抽样前 10 条快速验证

```bash
bash scripts/evaluate.sh --max-samples 10
```

### 4) 使用已有服务（不启动 docker）

```bash
bash scripts/evaluate.sh --no-start --base-url http://127.0.0.1:18081
```

## 输出产物

- 报告：`var/data/reports/baseline-YYYY-MM-DD.md`
- 明细指标：`var/data/reports/metrics-YYYYMMDD-HHMMSS.json`

报告最少包含三列核心数字：

- 改写前均值 `score_before_avg`
- 改写后均值 `score_after_avg`
- 降幅均值 `score_delta_avg = before - after`

## 可调参数

- `--dataset <path>`: 指定数据集
- `--profile <name>`: 覆盖所有样本 profile
- `--wait-seconds <n>`: 单样本超时
- `--max-samples <n>`: 限制样本数
- `--no-start`: 不自动起 docker
- `--keep-stack`: 保留 docker 环境

## 注意事项

- 未配置外部 detector 时，系统会走当前本地+fallback scorer 路径；报告仍可用于回归比较，但不代表第三方检测器真值。
- 若要接真实 detector（如 Binoculars 封装的 OpenAI-compatible endpoint），在环境变量里配置：
  - `NATURALIZE_PROVIDERS_DETECTOR_BASE_URL`
  - `NATURALIZE_PROVIDERS_DETECTOR_API_KEY`
  - `NATURALIZE_PROVIDERS_DETECTOR_MODEL`
