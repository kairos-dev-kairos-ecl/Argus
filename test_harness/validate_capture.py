"""
validate_capture.py — ClickHouse signal-capture validator for Argus harness

Connects to ClickHouse on localhost:9001 (test stack port), queries the
signals table for the most recent harness run (app_id='harness-qwen',
last 10 minutes), and produces a per-layer coverage report.

Exits 0 if all expected layers have at least one row.
Exits 1 if any expected layer is missing.

Writes report to test_harness/validation_results.md.

Usage:
    python test_harness/validate_capture.py
"""

import json
import os
import sys
from datetime import datetime, timezone
from pathlib import Path

import clickhouse_connect

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
CH_HOST = os.environ.get("CH_HOST", "localhost")
CH_PORT = int(os.environ.get("CH_PORT", "8124"))   # HTTP port (test stack)
CH_USER = os.environ.get("CH_USER", "default")
CH_PASSWORD = os.environ.get("CH_PASSWORD", "")
CH_DB = os.environ.get("CH_DB", "default")
APP_ID = "harness-qwen"
LOOKBACK_MINUTES = 10

EXPECTED_LAYERS_PATH = Path(__file__).parent / "expected_layers.json"
RESULTS_PATH = Path(__file__).parent / "validation_results.md"

# Layer names for display
LAYER_NAMES = {
    1: "L1  Hardware",
    2: "L2  Model Weights",
    3: "L3  Tokenizer",
    4: "L4  Transformer",
    5: "L5  Output Decoding",
    6: "L6  Safety",
    7: "L7  RAG Retrieval",
    8: "L8  Agents",
    9: "L9  Network/Gateway",
    10: "L10 Application",
    11: "LDec Decision Log",
}

# Layer-specific fields to check fill-rate (columns that the harness populates)
LAYER_FILL_FIELDS = {
    1:  ["ctx_l1_cpu_usage_pct", "ctx_l1_memory_used_mb"],
    2:  ["ctx_l2_model_id", "ctx_l2_quantization"],
    3:  ["ctx_l3_input_tokens", "ctx_l3_total_tokens"],
    4:  ["ctx_l4_sequence_length", "ctx_l4_batch_size"],
    5:  ["ctx_l5_output_tokens", "ctx_l5_finish_reason"],
    6:  ["ctx_l6_safety_score", "ctx_l6_safe"],
    8:  ["ctx_l8_step_number"],
    9:  ["ctx_l9_method", "ctx_l9_path", "ctx_l9_status_code"],
    10: ["ctx_l10_user_id", "ctx_l10_session_id"],
    11: ["ctx_ldec_decision", "ctx_ldec_confidence"],
}


def connect() -> clickhouse_connect.driver.client.Client:
    """Connect to ClickHouse via HTTP interface."""
    return clickhouse_connect.get_client(
        host=CH_HOST,
        port=CH_PORT,
        username=CH_USER,
        password=CH_PASSWORD,
        database=CH_DB,
    )


def run_validation(client) -> dict:
    """Run all validation queries and return results dict."""
    results = {}

    # 1. Total signals
    row = client.query(
        f"""
        SELECT count()
        FROM signals
        WHERE app_id = '{APP_ID}'
          AND timestamp > now() - INTERVAL {LOOKBACK_MINUTES} MINUTE
        """
    ).result_rows
    total = row[0][0] if row else 0
    results["total_signals"] = total

    # 2. Per-layer coverage
    rows = client.query(
        f"""
        SELECT layer, count() AS cnt
        FROM signals
        WHERE app_id = '{APP_ID}'
          AND timestamp > now() - INTERVAL {LOOKBACK_MINUTES} MINUTE
        GROUP BY layer
        ORDER BY layer
        """
    ).result_rows
    results["per_layer"] = {int(r[0]): int(r[1]) for r in rows}

    # 3. Per-layer field fill-rate (only for layers that appear in the data)
    fill_rates = {}
    for layer, fields in LAYER_FILL_FIELDS.items():
        if layer not in results["per_layer"]:
            continue
        layer_fill = {}
        for field in fields:
            try:
                r = client.query(
                    f"""
                    SELECT
                        countIf(isNotNull({field})) AS filled,
                        count() AS total
                    FROM signals
                    WHERE app_id = '{APP_ID}'
                      AND layer = {layer}
                      AND timestamp > now() - INTERVAL {LOOKBACK_MINUTES} MINUTE
                    """
                ).result_rows
                if r:
                    filled, ttl = int(r[0][0]), int(r[0][1])
                    layer_fill[field] = round(filled / ttl, 3) if ttl else 0.0
            except Exception as exc:
                # Column may not exist in this schema version
                layer_fill[field] = f"ERROR: {exc}"
        fill_rates[layer] = layer_fill
    results["fill_rates"] = fill_rates

    # 4. Trace correlation — each trace_id should have ~10 signals (L7 skipped)
    rows = client.query(
        f"""
        SELECT trace_id, count() AS cnt
        FROM signals
        WHERE app_id = '{APP_ID}'
          AND timestamp > now() - INTERVAL {LOOKBACK_MINUTES} MINUTE
        GROUP BY trace_id
        ORDER BY cnt DESC
        """
    ).result_rows
    results["traces"] = [(r[0], int(r[1])) for r in rows]

    # 5. Enrichment markers — check baseline_zscore and correlation_depth on L5/L6
    for col in ["baseline_zscore", "correlation_depth"]:
        try:
            r = client.query(
                f"""
                SELECT countIf(isNotNull({col})) AS filled, count() AS total
                FROM signals
                WHERE app_id = '{APP_ID}'
                  AND layer IN (5, 6)
                  AND timestamp > now() - INTERVAL {LOOKBACK_MINUTES} MINUTE
                """
            ).result_rows
            if r:
                filled, ttl = int(r[0][0]), int(r[0][1])
                results[f"enrichment_{col}"] = {"filled": filled, "total": ttl}
        except Exception:
            results[f"enrichment_{col}"] = {"filled": 0, "total": 0, "note": "column absent"}

    return results


def load_expected() -> dict:
    with open(EXPECTED_LAYERS_PATH) as f:
        return json.load(f)


def build_report(results: dict, expected: dict) -> tuple[str, bool]:
    """Return (markdown_report, passed)."""
    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")
    lines = []
    lines.append(f"# Argus Harness — Signal Capture Report")
    lines.append(f"")
    lines.append(f"**Generated:** {now}  ")
    lines.append(f"**App ID:** `{APP_ID}`  ")
    lines.append(f"**Lookback:** last {LOOKBACK_MINUTES} minutes  ")
    lines.append(f"")
    lines.append(f"## Summary")
    lines.append(f"")
    lines.append(f"| Metric | Value |")
    lines.append(f"|--------|-------|")
    lines.append(f"| Total signals captured | {results['total_signals']} |")
    lines.append(f"| Distinct traces | {len(results['traces'])} |")
    lines.append(f"")

    # Per-layer coverage table
    lines.append(f"## Per-Layer Coverage")
    lines.append(f"")
    lines.append(f"| Layer | Name | Expected | Captured | Status |")
    lines.append(f"|-------|------|----------|----------|--------|")

    all_pass = True
    per_layer = results["per_layer"]
    expected_layers = {e["layer"]: e for e in expected["layers"]}

    for layer_info in expected["layers"]:
        layer_num = layer_info["layer"]
        name = layer_info["name"]
        is_expected = layer_info["expected"]
        captured = per_layer.get(layer_num, 0)

        if is_expected:
            if captured > 0:
                status = "PASS"
            else:
                status = "FAIL"
                all_pass = False
        else:
            status = "SKIP" if captured == 0 else "UNEXPECTED"

        lines.append(f"| {layer_num} | {name} | {'Yes' if is_expected else 'No'} | {captured} | {status} |")

    lines.append(f"")

    # Fill-rate table
    lines.append(f"## Field Fill-Rate (expected fields only)")
    lines.append(f"")
    lines.append(f"| Layer | Field | Fill Rate |")
    lines.append(f"|-------|-------|-----------|")

    for layer, fields in results["fill_rates"].items():
        for field, rate in fields.items():
            if isinstance(rate, float):
                rate_str = f"{rate:.1%}"
                flag = " LOW" if rate < 1.0 else ""
            else:
                rate_str = str(rate)
                flag = " ERROR"
            lines.append(f"| {layer} | `{field}` | {rate_str}{flag} |")

    lines.append(f"")

    # Trace correlation
    lines.append(f"## Trace Correlation")
    lines.append(f"")
    lines.append(f"Expected: {expected['expected_traces']} traces × "
                 f"~{expected['expected_layers_per_trace']} signals each  ")
    lines.append(f"")
    lines.append(f"| Trace ID | Signals |")
    lines.append(f"|----------|---------|")
    for trace_id, cnt in results["traces"]:
        lines.append(f"| `{trace_id[:16]}…` | {cnt} |")

    if not results["traces"]:
        lines.append(f"| (none found) | 0 |")

    lines.append(f"")

    # Enrichment markers
    lines.append(f"## Enrichment Markers (baseline_zscore, correlation_depth)")
    lines.append(f"")
    lines.append(f"| Column | Filled | Total | Fill Rate |")
    lines.append(f"|--------|--------|-------|-----------|")
    for col in ["baseline_zscore", "correlation_depth"]:
        key = f"enrichment_{col}"
        if key in results:
            d = results[key]
            filled = d.get("filled", 0)
            total = d.get("total", 0)
            rate = f"{filled/total:.1%}" if total else "n/a"
            note = d.get("note", "")
            lines.append(f"| `{col}` | {filled} | {total} | {rate} {note} |")

    lines.append(f"")
    lines.append(f"## Result")
    lines.append(f"")
    if all_pass:
        lines.append(f"**PASSED** — all expected layers captured.")
    else:
        lines.append(f"**FAILED** — one or more expected layers have zero rows. "
                     f"Check Argus ingest logs and pipeline health.")

    return "\n".join(lines), all_pass


def main() -> int:
    print(f"Connecting to ClickHouse at {CH_HOST}:{CH_PORT}…")
    try:
        client = connect()
        client.ping()
        print("Connected.")
    except Exception as exc:
        print(f"[ERROR] Cannot connect to ClickHouse: {exc}")
        print(f"  Expected test stack on port {CH_PORT}. Is it running?")
        return 1

    expected = load_expected()

    print(f"Querying signals table for app_id='{APP_ID}' last {LOOKBACK_MINUTES} minutes…")
    try:
        results = run_validation(client)
    except Exception as exc:
        print(f"[ERROR] Validation query failed: {exc}")
        return 1

    report, passed = build_report(results, expected)

    # Print to stdout
    print()
    print(report)
    print()

    # Write to file
    RESULTS_PATH.write_text(report, encoding="utf-8")
    print(f"Report written to: {RESULTS_PATH}")

    if passed:
        print("RESULT: PASSED")
        return 0
    else:
        print("RESULT: FAILED — see missing layers above")
        return 1


if __name__ == "__main__":
    sys.exit(main())
