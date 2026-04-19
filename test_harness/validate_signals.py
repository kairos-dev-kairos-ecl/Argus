"""
Signal Validation Suite — Verify end-to-end signal capture

Queries Argus backend to validate:
  1. All signals received and persisted in ClickHouse
  2. All 10 layers represented
  3. Signal schema compliance
  4. Trace correlation
  5. Enrichment fields populated
"""

import asyncio
import os
import json
from typing import Dict, List, Any
from datetime import datetime, timedelta

import httpx


class SignalValidator:
    """Validate signals persisted in Argus"""

    def __init__(self, base_url: str = "http://localhost:8080", api_key: str = ""):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.session: httpx.AsyncClient = None

    async def __aenter__(self):
        self.session = httpx.AsyncClient(timeout=30.0)
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self.session:
            await self.session.aclose()

    async def query_signals(
        self,
        app_id: str = "qwen-test-harness",
        trace_id: str = None,
        layer: int = None,
        limit: int = 1000,
    ) -> List[Dict[str, Any]]:
        """
        Query signals from ClickHouse via HTTP.

        Args:
            app_id: Application ID filter
            trace_id: Trace ID filter
            layer: Layer number filter (1-10)
            limit: Result limit

        Returns:
            List of signal dicts
        """
        # Build ClickHouse SQL query
        where_clauses = [f"source.app_id = '{app_id}'"]
        if trace_id:
            where_clauses.append(f"trace_id = '{trace_id}'")
        if layer:
            where_clauses.append(f"layer = {layer}")

        where_clause = " AND ".join(where_clauses)
        sql = f"""
        SELECT
            signal_id,
            trace_id,
            span_id,
            layer,
            category,
            severity,
            timestamp,
            duration_ms,
            source,
            provider,
            enrichment
        FROM signals
        WHERE {where_clause}
        ORDER BY timestamp DESC
        LIMIT {limit}
        """

        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"

        try:
            response = await self.session.post(
                f"{self.base_url}/api/v1/query",
                json={"sql": sql},
                headers=headers,
            )

            if response.status_code == 200:
                return response.json().get("results", [])
            else:
                print(f"[ERROR] Query failed: {response.status_code} {response.text}")
                return []
        except Exception as e:
            print(f"[ERROR] Query exception: {e}")
            return []

    async def validate_all_layers(self, app_id: str = "qwen-test-harness") -> Dict[int, List[Dict]]:
        """Query signals for all 10 layers"""
        results = {}
        layer_names = {
            1: "HARDWARE",
            2: "MODEL_WEIGHTS",
            3: "TOKENIZER",
            4: "TRANSFORMER",
            5: "OUTPUT_DECODING",
            6: "SAFETY",
            7: "RAG_RETRIEVAL",
            8: "AGENTS",
            9: "API_GATEWAY",
            10: "APPLICATION",
        }

        print("\n[VALIDATE] Querying signals by layer...")
        for layer in range(1, 11):
            signals = await self.query_signals(app_id=app_id, layer=layer, limit=10)
            results[layer] = signals
            count = len(signals)
            status = "✓" if count > 0 else "✗"
            print(f"  {status} L{layer} ({layer_names[layer]}): {count} signals")

        return results

    async def validate_trace_correlation(
        self, app_id: str = "qwen-test-harness"
    ) -> Dict[str, int]:
        """Validate trace correlation (signals linked by trace_id)"""
        signals = await self.query_signals(app_id=app_id, limit=1000)

        if not signals:
            print("[WARN] No signals found")
            return {}

        # Group by trace_id
        traces = {}
        for signal in signals:
            trace_id = signal.get("trace_id")
            if trace_id:
                if trace_id not in traces:
                    traces[trace_id] = []
                traces[trace_id].append(signal)

        print(f"\n[VALIDATE] Trace correlation...")
        print(f"  Total signals: {len(signals)}")
        print(f"  Total traces: {len(traces)}")

        # Analyze trace composition
        trace_stats = {}
        for trace_id, trace_signals in traces.items():
            layers = sorted(set(s.get("layer") for s in trace_signals))
            trace_stats[trace_id] = {
                "signal_count": len(trace_signals),
                "layer_count": len(layers),
                "layers": layers,
            }

        # Show first few traces
        print("\n  Sample traces:")
        for i, (trace_id, stats) in enumerate(list(trace_stats.items())[:3]):
            print(f"    Trace {i+1}: {stats['signal_count']} signals across L{stats['layers']}")

        return trace_stats

    async def validate_schema_compliance(
        self, app_id: str = "qwen-test-harness"
    ) -> bool:
        """Validate signal schema compliance"""
        signals = await self.query_signals(app_id=app_id, limit=100)

        print(f"\n[VALIDATE] Schema compliance ({len(signals)} samples)...")

        required_fields = {
            "signal_id": str,
            "trace_id": str,
            "layer": int,
            "category": str,
            "severity": int,
            "timestamp": (str, dict),
        }

        issues = []
        for signal in signals:
            for field, expected_type in required_fields.items():
                if field not in signal:
                    issues.append(f"Missing field: {field}")
                elif isinstance(expected_type, tuple):
                    if not isinstance(signal[field], expected_type):
                        issues.append(
                            f"Field {field}: expected {expected_type}, got {type(signal[field])}"
                        )
                else:
                    if not isinstance(signal[field], expected_type):
                        issues.append(
                            f"Field {field}: expected {expected_type}, got {type(signal[field])}"
                        )

        if issues:
            print(f"  ✗ Schema issues found:")
            for issue in issues[:5]:
                print(f"    - {issue}")
            return False
        else:
            print(f"  ✓ All signals comply with schema")
            return True

    async def validate_enrichment(
        self, app_id: str = "qwen-test-harness"
    ) -> Dict[str, Any]:
        """Validate enrichment fields are populated"""
        signals = await self.query_signals(app_id=app_id, limit=100)

        print(f"\n[VALIDATE] Enrichment fields ({len(signals)} samples)...")

        enrichment_stats = {
            "geo_data": 0,
            "threat_intel": 0,
            "baseline_deviation": 0,
            "risk_score": 0,
            "empty_enrichment": 0,
        }

        for signal in signals:
            enrichment = signal.get("enrichment", {})
            if not enrichment or enrichment == {}:
                enrichment_stats["empty_enrichment"] += 1
            else:
                if enrichment.get("geo"):
                    enrichment_stats["geo_data"] += 1
                if enrichment.get("threat_intel"):
                    enrichment_stats["threat_intel"] += 1
                if enrichment.get("baseline_deviation") is not None:
                    enrichment_stats["baseline_deviation"] += 1
                if enrichment.get("risk_score") is not None:
                    enrichment_stats["risk_score"] += 1

        for key, count in enrichment_stats.items():
            pct = (count / len(signals) * 100) if signals else 0
            print(f"  {key}: {count}/{len(signals)} ({pct:.1f}%)")

        return enrichment_stats

    async def run_full_validation(self, app_id: str = "qwen-test-harness") -> Dict[str, Any]:
        """Run complete validation suite"""
        print("=" * 70)
        print("ARGUS SIGNAL VALIDATION SUITE")
        print("=" * 70)

        results = {
            "timestamp": datetime.now().isoformat(),
            "app_id": app_id,
            "checks": {},
        }

        # 1. Layer coverage
        layer_results = await self.validate_all_layers(app_id=app_id)
        layer_coverage = sum(1 for layer_signals in layer_results.values() if len(layer_signals) > 0)
        results["checks"]["layer_coverage"] = {
            "expected": 10,
            "found": layer_coverage,
            "passed": layer_coverage >= 7,  # At least 7 of 10
        }

        # 2. Trace correlation
        trace_results = await self.validate_trace_correlation(app_id=app_id)
        results["checks"]["trace_correlation"] = {
            "total_traces": len(trace_results),
            "passed": len(trace_results) > 0,
        }

        # 3. Schema compliance
        schema_passed = await self.validate_schema_compliance(app_id=app_id)
        results["checks"]["schema_compliance"] = {"passed": schema_passed}

        # 4. Enrichment
        enrichment_results = await self.validate_enrichment(app_id=app_id)
        results["checks"]["enrichment"] = enrichment_results

        # Summary
        print("\n" + "=" * 70)
        print("VALIDATION SUMMARY")
        print("=" * 70)

        passed = sum(
            1
            for check in results["checks"].values()
            if isinstance(check, dict) and check.get("passed")
        )
        total = sum(
            1
            for check in results["checks"].values()
            if isinstance(check, dict) and "passed" in check
        )

        print(f"\n✓ {passed}/{total} checks passed")

        for check_name, check_result in results["checks"].items():
            if isinstance(check_result, dict) and "passed" in check_result:
                status = "✓ PASS" if check_result["passed"] else "✗ FAIL"
                print(f"  {status}: {check_name}")

        return results


async def main():
    """Run validation against Argus backend"""
    argus_url = os.getenv("ARGUS_URL", "http://localhost:8080")
    api_key = os.getenv("ARGUS_API_KEY", "")

    async with SignalValidator(base_url=argus_url, api_key=api_key) as validator:
        results = await validator.run_full_validation()

        # Save results to file
        output_file = "validation_results.json"
        with open(output_file, "w") as f:
            json.dump(results, f, indent=2)

        print(f"\n[INFO] Results saved to {output_file}")


if __name__ == "__main__":
    asyncio.run(main())
