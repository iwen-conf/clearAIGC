既然明确了不写代码，我们直接输出 Eino 框架下的**节点配置矩阵（Configuration Matrix）**。

在工业级 Eino 实践中，参数配置必须与业务逻辑绝对解耦。我们将这三个 Agent 的配置抽象为标准化的规格说明书。请注意一个核心的工程原则：**在我们的设计中，只有主控节点（Coordinator）是真正的 ReAct Agent（具备思考和调用工具的能力），而两个 Worker 节点应当被降级配置为严格的 Chain（大模型 + 结构化输出解析器），以此彻底封死它们发散和产生幻觉的可能，并大幅降低 Token 消耗。**

以下是你在 Eino 框架中需要填写的全套参数规格：

---

### 一、 主控节点：RewriteCoordinator (流控官)

**1. 节点基础配置 (Eino Node Setup)**
* **Node Type (节点类型):** `ReAct Agent` (带有状态机的循环网络)
* **Max Iterations (最大内部循环次数):** `3` (硬性防死循环熔断)
* **Input Schema (输入结构):** `{"chunk_id": "string", "text": "string", "context_summary": "string"}`

**2. System Prompt (系统提示词)**
> **[Role]**
> You are the Workflow Controller for an academic text processing pipeline. Your ONLY objective is to analyze the provided text chunk, identify AI-generated stylistic signatures (high Perplexity/PPL and uniform Burstiness), and delegate specific modification tasks to downstream workers.
> 
> **[Constraints]**
> 1. DO NOT rewrite the text yourself.
> 2. You are a machine gateway. Do not output conversational text, greetings, or explanations.
> 3. Your output must strictly be the invocation of available tools.
> 
> **[Workflow]**
> 1. Analyze the input `text`.
> 2. If vocabulary is generic (e.g., "crucial", "delve into"), invoke `call_lexical_mutator`.
> 3. If sentence lengths are overly uniform, invoke `call_syntax_rebuilder`.
> 4. If both are required, plan the execution sequentially.

**3. Bound Tools / Functions (绑定的工具列表)**
在 Eino 中，这些 Tool 会被转换为大模型的 Function Calling 接口：

* **Tool 1: `call_lexical_mutator`**
    * **Description:** "Sends the text to the Lexical Analyst to replace high-frequency AI vocabulary with long-tail academic synonyms without altering sentence structure."
    * **Parameters:** `{"target_text": "string"}`
* **Tool 2: `call_syntax_rebuilder`**
    * **Description:** "Sends the text to the Structure Architect to break topological uniformity by splitting long sentences, merging short ones, or shifting active/passive voices."
    * **Parameters:** `{"target_text": "string"}`
* **Tool 3: `submit_final_chunk`**
    * **Description:** "Marks the processing for this chunk as complete and submits the rewritten text back to the main DAG pipeline."
    * **Parameters:** `{"rewritten_text": "string", "modifications_made": ["string"]}`

---

### 二、 词汇工作节点：LexicalMutator (词汇分析师)

**1. 节点基础配置 (Eino Node Setup)**
* **Node Type (节点类型):** `Chain` (线性执行，无 ReAct 循环)
* **Output Parser (输出解析器):** `JSON Structured Output` (强制反序列化为 Go 结构体)
* **Temperature (温度参数):** `0.2` (词汇替换需要高确定性和收敛性)

**2. System Prompt (系统提示词)**
> **[Role]**
> You are a Lexical Analyst operating as a headless microservice. Your task is to dilute the probability distribution (PPL) of the provided academic text.
> 
> **[Directives]**
> 1. Scan the text for overused AI-generated words (e.g., "Moreover", "It is worth noting", "Comprehensive", "Pivotal").
> 2. Replace them with precise, human-like, long-tail academic terminology specific to the context.
> 3. **CRITICAL BOUNDARY:** DO NOT add, delete, or move any commas, periods, or clauses. Maintain the exact original Abstract Syntax Tree (AST) of the sentence.
> 4. Ignore words wrapped in `__TERM__` markers.
> 
> **[Output Format]**
> You must output a precise Diff-Patch array in JSON format. Do not return the full text.

**3. Expected Function Output Schema (必须强制大模型输出的 JSON 格式)**
```json
{
  "type": "object",
  "properties": {
    "patches": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "original_word": {"type": "string"},
          "replacement_word": {"type": "string"},
          "reasoning": {"type": "string", "description": "Brief reason for replacement (max 5 words)"}
        },
        "required": ["original_word", "replacement_word"]
      }
    }
  }
}
```
*(注：这种 Diff 输出是工业级的精髓，后端通过纯代码应用这个 Patch 数组，能节省至少 60% 的 Output Token 开销。)*

---

### 三、 句法工作节点：SyntaxRebuilder (结构架构师)

**1. 节点基础配置 (Eino Node Setup)**
* **Node Type (节点类型):** `Chain` (同样剥夺其 ReAct 权限)
* **Output Parser:** `JSON Structured Output`
* **Temperature:** `0.7` (结构重组需要较高的发散性和创造力)

**2. System Prompt (系统提示词)**
> **[Role]**
> You are a Structure Architect. Your objective is to inject "Burstiness" (variance in sentence length and topological complexity) into the provided text to bypass AI stylistic detection.
> 
> **[Directives]**
> 1. Analyze the rhythm of the text. AI writes in uniform, balanced, multi-clause sentences.
> 2. **Disrupt this uniformity:** Forcefully split long compound sentences into stark, punchy short sentences. 
> 3. Consolidate fragmented short sentences into complex, asymmetrical structures using semicolons or em-dashes.
> 4. Invert grammatical structures (e.g., shift passive voice to active, use fronted adverbials).
> 5. **CRITICAL BOUNDARY:** You must strictly preserve the logical sequence and the academic validity of the arguments. Do not alter facts.

**3. Expected Function Output Schema**
```json
{
  "type": "object",
  "properties": {
    "rebuilt_text": {
      "type": "string",
      "description": "The fully restructured text block"
    },
    "burstiness_metrics": {
      "type": "object",
      "properties": {
        "longest_sentence_word_count": {"type": "integer"},
        "shortest_sentence_word_count": {"type": "integer"}
      }
    }
  },
  "required": ["rebuilt_text"]
}
```

---

这套配置将不确定性（幻觉）锁死在了最底层的 JSON Schema 里。
