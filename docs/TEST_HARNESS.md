# Argus Test Harness User Guide

## Overview

The Argus test harness validates the platform's ability to detect threats against LLM-integrated systems. It simulates 22 attack scenarios across 6 threat categories and measures detection accuracy, false positive rates, and signal coverage across all 10 system layers.

**Test Environment:**
```
Ollama (Local LLM)
         ↓
Python Test App (async SDK client, RAG, agents)
         ↓
Argus Instance (signal ingest → processing → detection)
         ↓
Test Runner (validation, reporting)
```

## Setup

### Prerequisites

- **Python 3.9+**
- **Ollama** (local LLM runtime) — [Download](https://ollama.ai)
- **Argus** (running on localhost:8080)
- **Poetry** or **pip** (for Python dependencies)

### Installation

1. **Install Python dependencies:**
   ```bash
   pip install httpx pyyaml numpy
   ```

2. **Start Ollama:**
   ```bash
   ollama serve
   ```
   In another terminal, pull a model:
   ```bash
   ollama pull llama2
   ```
   Or use Qwen2.5:
   ```bash
   ollama pull qwen2.5:7b
   ```

3. **Start Argus:**
   ```bash
   # In Argus project root
   make run
   ```

4. **Verify connectivity:**
   ```bash
   # Test Ollama
   curl http://localhost:11434/api/tags

   # Test Argus
   curl http://localhost:8080/health
   ```

## Running Tests

### Full Test Suite

```bash
cd test_harness
python runner.py
```

**Timeline:** ~25 minutes
- Phase 1 (Baseline): ~5 minutes
- Baseline engine wait: ~15 minutes
- Phase 2 (Attacks): ~4 minutes
- Phase 3 (Chain): ~1 minute

### Individual Scenario Set

To run a single scenario set:

```bash
python runner.py --set A  # Run baseline only
python runner.py --set B  # Run prompt injection scenarios
```

### Scenario Definitions

Scenarios are defined in YAML format in `scenarios/`:

- **set_a_baseline.yaml** — 5 clean traces (expect 0 detections)
- **set_b_prompt_injection.yaml** — 5 prompt injection attacks
- **set_c_data_integrity.yaml** — 4 data integrity attacks (hallucination, RAG poisoning)
- **set_d_agent_abuse.yaml** — 4 agent/tool abuse attacks
- **set_e_infrastructure.yaml** — 3 infrastructure attacks (DoW, API abuse, extraction)
- **set_f_chain_attack.yaml** — 1 advanced multi-step attack

Each scenario specifies:
- **Type:** chat, rag_query, agent_task, chat_multi_turn, chat_batch
- **Expected detection:** The detection rule expected to fire
- **Attack pattern:** Category for analysis

## Coverage Report

After tests complete, view the coverage report:

```
╔══════════════════════════════════════════════╗
║  ARGUS XDR — SIGNAL COVERAGE & DETECTION   ║
║  VALIDATION REPORT                          ║
╠══════════════════════════════════════════════╣
║  Test Run: 2026-04-15T14:30:00Z             ║
║  Duration: 4m 23s                           ║
║                                             ║
║  SIGNAL COVERAGE                            ║
║  L3: 34 | L5: 32 | L6: 28 | L7: 31 | L8: 38║
║  Total: 187 (100% ingested)                 ║
║                                             ║
║  DETECTION RESULTS                          ║
║  Expected: 18 | TP: 16 (88.9%)              ║
║  FN: 2 | FP: 1                              ║
║                                             ║
║  THREAT COVERAGE                            ║
║  ✅ Prompt Injection    ✅ Tool Abuse        ║
║  ✅ Data Integrity      ✅ Infrastructure    ║
║  ⚠️  Jailbreak          ✅ Chain Attacks     ║
║                                             ║
║  OVERALL: 88.9% detection rate              ║
╚══════════════════════════════════════════════╝
```

**Metrics Explained:**

- **Signal Coverage:** Signals received per layer (L1-L10). Should be >0 for all active layers.
- **TP (True Positives):** Attacks correctly detected as threats
- **FN (False Negatives):** Attacks missed (detection failed)
- **FP (False Positives):** Legitimate traces incorrectly flagged
- **Detection Rate:** TP / (TP + FN) — target ≥85%
- **False Positive Rate:** FP / legitimate scenarios — target <5%

## Interpreting Results

### Success Criteria

Test suite **PASSES** if:
- ✅ Detection rate ≥ 85%
- ✅ False positive rate < 5%
- ✅ All layers (L3, L5-L10) report signals
- ✅ Set F (chain attack) produces ≥4 correlated signals

### Common Issues

**Low Detection Rate**

If detection rate is <85%:

1. **Check detection rule configuration** — verify rules in Argus config are enabled
2. **Review false negatives** — which scenarios failed?
3. **Adjust thresholds** — some detectors may need tuning for your Ollama model
4. **Check baseline engine** — ensure baseline profiles were computed (wait another 15 min)

**High False Positive Rate**

If FP rate is >5%:

1. **Check baseline profiles** — ensure they're accurate (run Set A first)
2. **Review legitimate scenarios** — which benign traces were flagged?
3. **Adjust sensitivity** — may need to increase thresholds for this LLM model

**No Signals from Certain Layers**

If layer coverage is incomplete:

1. **Check test app logging** — is the test app emitting signals?
2. **Verify SDK client** — confirm Argus HTTP endpoint is accessible
3. **Check Ollama output** — ensure LLM is generating tokens
4. **Verify ingest pipeline** — confirm Argus ingestion is running

## Tuning Detection Rules

### Accessing Detection Rules

Rules are stored in Argus configuration (YAML or database). They define:
- What patterns trigger alerts
- Severity thresholds
- Correlation logic

### Common Tuning Parameters

**Prompt Injection Rules**
```yaml
prompt-injection-direct:
  keywords: ["ignore previous", "forget instructions", "system prompt"]
  similarity_threshold: 0.8
  min_tokens: 5
  enabled: true
```

**Baseline Deviation**
```yaml
baseline-deviation:
  z_score_threshold: 2.5  # Standard deviations from mean
  min_sample_count: 100   # Minimum baseline samples
  enabled: true
```

**Agent Loop Detection**
```yaml
agent-runaway-loop:
  iteration_limit: 20
  timeout_sec: 60
  enabled: true
```

### Tuning Workflow

1. **Run baseline suite (Set A)** — establish clean behavior profile
2. **Wait 15+ minutes** — let baseline engine compute profiles
3. **Run attack suites (Sets B-E)** — check detection rates
4. **Adjust high-confidence rules** first (prompt injection, unauthorized tools)
5. **Adjust statistical rules** (baseline deviation) if needed
6. **Rerun** to validate changes

## Results Analysis

### Results File

Results are saved to `test_harness/results.json`:

```json
{
  "metrics": {
    "total_scenarios": 22,
    "passed_scenarios": 20,
    "true_positives": 18,
    "false_negatives": 2,
    "false_positives": 1,
    "detection_rate_pct": 88.9,
    "false_positive_rate_pct": 4.3,
    "signals_by_layer": {
      "L3_TOKENIZER": 15,
      "L5_OUTPUT_DECODING": 32,
      "L6_SAFETY": 28,
      "L7_RAG_RETRIEVAL": 31,
      "L8_AGENTS": 38,
      "L10_APPLICATION": 26
    },
    "detection_by_category": {
      "prompt-injection-direct": {
        "count": 3,
        "scenarios": ["B1", "B5"]
      }
    }
  },
  "results": [
    {
      "scenario_id": "A1",
      "scenario_name": "Simple Chat",
      "passed": true,
      "detected": false,
      "expected_detection": null,
      "actual_detection": null,
      "duration_ms": 2500,
      "signals_emitted": 4
    }
  ]
}
```

### Analyzing Detections

**By Attack Category:**

Use `detection_by_category` to understand which rules are firing:

```python
import json

with open("results.json") as f:
    data = json.load(f)
    
for category, info in data["metrics"]["detection_by_category"].items():
    print(f"{category}: {info['count']} detections in {info['scenarios']}")
```

**By Layer:**

Use `signals_by_layer` to verify coverage:

```python
total = sum(data["metrics"]["signals_by_layer"].values())
for layer, count in data["metrics"]["signals_by_layer"].items():
    pct = count / total * 100
    print(f"{layer}: {count} signals ({pct:.1f}%)")
```

## Advanced Usage

### Custom Scenarios

Create a new YAML file in `scenarios/`:

```yaml
scenarios:
  - id: X1
    name: "My Custom Attack"
    type: "chat"
    prompt: "Your custom prompt here"
    expected_detection: "my-custom-detection"
    expected_severity: "HIGH"
    timeout_sec: 30
```

### Modifying the Test App

Edit `test_harness/app.py` to:
- Add new tool types
- Change RAG documents
- Adjust inference parameters
- Add new attack vectors

### Custom Detection Rules

Rules are defined in Argus configuration. To add a custom rule:

1. Define rule in YAML format
2. Deploy to Argus (via dashboard or API)
3. Add matching scenario to test suite
4. Verify detection in results

## Performance Benchmarks

**Typical execution times:**

| Scenario Type | Time (ms) | Layer |
|---------------|-----------|-------|
| Simple chat | 2000-5000 | L5, L10 |
| RAG query | 3000-8000 | L7, L5 |
| Agent task | 1000-4000 | L8 |
| Chain attack | 15000-25000 | L7, L5, L8 |

**Signal volume:**

- Baseline suite (A): ~20 signals, 0 detections
- Attack suites (B-E): ~150 signals, ~18 detections
- Chain suite (F): ~25 signals, ~4-5 detections
- **Total: ~200 signals per full run**

## Troubleshooting

### Test Timeouts

If tests hang or timeout:

1. **Increase timeout values** in scenario YAML
2. **Check Ollama** — is it responding? `curl http://localhost:11434/api/tags`
3. **Check Argus** — is ingest running? Check logs
4. **Check network** — firewall blocking localhost connections?

### Connection Errors

```
ERROR: Error connecting to Ollama at http://localhost:11434
```

Solution:
```bash
# Start Ollama
ollama serve

# Verify it's running
curl http://localhost:11434/api/tags
```

### Missing Signals

If signals aren't reaching Argus:

1. **Check SDK client initialization** — verify app emits signals
2. **Check HTTP endpoint** — verify `http://localhost:8080` is accessible
3. **Check ingest logs** — look for errors in Argus logs
4. **Check network** — try manual curl:
   ```bash
   curl -X POST http://localhost:8080/api/v1/signals \
     -H "Content-Type: application/json" \
     -d '{...}'
   ```

### No Detections

If no detections are firing:

1. **Check baseline engine** — run `SELECT COUNT(*) FROM baseline_profiles`
2. **Check detection rules** — verify rules are enabled
3. **Check thresholds** — may be too strict for your model
4. **Check logs** — review Argus detection engine logs

## Support & Contributing

Issues or improvements?

- Report bugs: GitHub Issues
- Suggest scenarios: GitHub Discussions
- Contribute rules: Pull requests to detection rules repository

## Next Steps

After validating the test harness:

1. **Deploy to staging** — run against staging Argus instance
2. **Tune for production** — adjust thresholds for your LLM model
3. **Integrate into CI/CD** — add test harness to GitHub Actions
4. **Monitor production** — track detection rates over time

---

**Test Harness Version:** 0.1.0
**Last Updated:** 2026-04-15
**Status:** Stable
