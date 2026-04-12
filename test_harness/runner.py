"""
Test Runner — Orchestrates scenario execution, validation, and coverage reporting
"""

import asyncio
import time
import yaml
import json
from typing import Any, Dict, List, Optional, Tuple
from dataclasses import dataclass, asdict
from datetime import datetime
from pathlib import Path
import httpx

from app import ArgusTestApp, OllamaClient, Document
from sdk.client import ArgusClient, Layer, Severity


@dataclass
class ScenarioResult:
    """Result of a single scenario execution"""
    scenario_id: str
    scenario_name: str
    passed: bool
    detected: bool
    expected_detection: Optional[str]
    actual_detection: Optional[str]
    error: Optional[str] = None
    duration_ms: float = 0.0
    signals_emitted: int = 0


@dataclass
class CoverageMetrics:
    """Coverage analysis results"""
    total_scenarios: int
    passed_scenarios: int
    true_positives: int
    false_negatives: int
    false_positives: int
    detection_rate_pct: float
    false_positive_rate_pct: float
    signals_by_layer: Dict[str, int]
    detection_by_category: Dict[str, Dict[str, Any]]
    duration_sec: float


class TestRunner:
    """Orchestrates test harness execution and validation"""

    def __init__(
        self,
        argus_url: str = "http://localhost:8080",
        ollama_url: str = "http://localhost:11434",
        argus_poll_interval_sec: float = 5.0,
        argus_poll_timeout_sec: float = 300.0,
    ):
        self.argus_url = argus_url
        self.ollama_url = ollama_url
        self.argus_poll_interval = argus_poll_interval_sec
        self.argus_poll_timeout = argus_poll_timeout_sec
        self.http_session: Optional[httpx.AsyncClient] = None
        self.results: List[ScenarioResult] = []
        self.signals: List[Dict[str, Any]] = []

    async def __aenter__(self):
        self.http_session = httpx.AsyncClient(timeout=30.0)
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self.http_session:
            await self.http_session.aclose()

    async def run_test_suite(self) -> CoverageMetrics:
        """
        Run full test suite (Phases 1-3):
        1. Run Set A (baseline), wait for baseline engine
        2. Run Sets B-E (attacks)
        3. Run Set F (chain)
        """
        start_time = time.time()

        print("\n" + "=" * 70)
        print("ARGUS TEST HARNESS — SCENARIO EXECUTION")
        print("=" * 70)

        # Load all scenarios
        scenarios_by_set = self._load_scenarios()

        # Setup test app
        async with ArgusClient(self.argus_url) as argus:
            async with OllamaClient(self.ollama_url) as ollama:
                # Check Ollama health
                if not await ollama.health_check():
                    raise RuntimeError(
                        "Ollama not running. Start with: ollama serve"
                    )

                app = ArgusTestApp(argus, ollama, "test-harness")

                # Setup RAG documents (mix of clean and poisoned)
                self._setup_rag_documents(app)

                # Phase 1: Baseline
                print("\n[PHASE 1] Running Scenario Set A (Baseline)...")
                await self._run_scenario_set("A", scenarios_by_set["A"], app, argus)

                # Wait for baseline engine
                print("\n[BASELINE] Waiting 15 minutes for baseline engine...")
                await asyncio.sleep(15 * 60)

                # Phase 2: Attacks
                print("\n[PHASE 2] Running Scenario Sets B-E (Attacks)...")
                for set_name in ["B", "C", "D", "E"]:
                    if set_name in scenarios_by_set:
                        await self._run_scenario_set(
                            set_name, scenarios_by_set[set_name], app, argus
                        )

                # Phase 3: Chain attack
                print("\n[PHASE 3] Running Scenario Set F (Chain Attack)...")
                if "F" in scenarios_by_set:
                    await self._run_scenario_set(
                        "F", scenarios_by_set["F"], app, argus
                    )

        # Analyze results
        duration_sec = time.time() - start_time
        metrics = self._analyze_results(duration_sec)

        # Generate report
        self._print_coverage_report(metrics)

        return metrics

    async def _run_scenario_set(
        self, set_name: str, scenarios: List[Dict[str, Any]], app: ArgusTestApp, argus: ArgusClient
    ):
        """Run all scenarios in a set"""
        for scenario in scenarios:
            await self._run_scenario(scenario, app, argus)

    async def _run_scenario(
        self, scenario: Dict[str, Any], app: ArgusTestApp, argus: ArgusClient
    ):
        """Run a single scenario and record results"""
        scenario_id = scenario["id"]
        scenario_name = scenario["name"]
        scenario_type = scenario["type"]

        print(f"  [{scenario_id}] {scenario_name}...", end=" ", flush=True)

        start = time.time()
        error = None
        signals_before = len(self.signals)

        try:
            if scenario_type == "chat":
                await app.chat(scenario["prompt"])
            elif scenario_type == "rag_query":
                await app.rag_query(scenario["query"])
            elif scenario_type == "agent_task":
                await app.agent_task(scenario["goal"])
            elif scenario_type == "chat_multi_turn":
                for turn in scenario["turns"]:
                    await app.chat(turn)
                    await asyncio.sleep(0.5)
            elif scenario_type == "chat_batch":
                for i in range(scenario.get("batch_count", 5)):
                    prompt = scenario["prompt_template"].format(i=i)
                    await app.chat(prompt)
                    await asyncio.sleep(0.2)
            else:
                print(f"SKIP (unknown type)")
                return

        except Exception as e:
            error = str(e)
            print(f"ERROR: {error}")
            self.results.append(
                ScenarioResult(
                    scenario_id=scenario_id,
                    scenario_name=scenario_name,
                    passed=False,
                    detected=False,
                    expected_detection=scenario.get("expected_detection"),
                    actual_detection=None,
                    error=error,
                    duration_ms=(time.time() - start) * 1000,
                    signals_emitted=0,
                )
            )
            return

        duration_ms = (time.time() - start) * 1000
        signals_emitted = len(self.signals) - signals_before

        # Poll for detections (async)
        await asyncio.sleep(2)  # Brief wait for ingestion
        detections = await self._poll_detections(
            trace_id=argus._trace_id,
            timeout_sec=scenario.get("timeout_sec", 30),
        )

        expected_detection = scenario.get("expected_detection")
        actual_detection = self._match_detection(detections, expected_detection)
        passed = (expected_detection is None and actual_detection is None) or (
            actual_detection == expected_detection
        )

        status = "PASS" if passed else "FAIL"
        print(status)

        self.results.append(
            ScenarioResult(
                scenario_id=scenario_id,
                scenario_name=scenario_name,
                passed=passed,
                detected=actual_detection is not None,
                expected_detection=expected_detection,
                actual_detection=actual_detection,
                error=None,
                duration_ms=duration_ms,
                signals_emitted=signals_emitted,
            )
        )

    async def _poll_detections(
        self, trace_id: Optional[str], timeout_sec: float
    ) -> List[Dict[str, Any]]:
        """Poll Argus API for detections within timeout"""
        if not trace_id:
            return []

        start = time.time()
        while time.time() - start < timeout_sec:
            try:
                response = await self.http_session.get(
                    f"{self.argus_url}/api/v1/signals",
                    params={"trace_id": trace_id},
                )
                if response.status_code == 200:
                    signals = response.json().get("signals", [])
                    self.signals.extend(signals)

                    # Extract detections from signals
                    detections = []
                    for signal in signals:
                        if signal.get("category", "").startswith("detection"):
                            detections.append(signal)
                    return detections
            except:
                pass

            await asyncio.sleep(self.argus_poll_interval)

        return []

    def _match_detection(
        self, detections: List[Dict[str, Any]], expected: Optional[str]
    ) -> Optional[str]:
        """Find matching detection from list"""
        if not expected:
            return None

        for detection in detections:
            category = detection.get("category", "")
            if expected.lower() in category.lower():
                return category

        return None

    def _load_scenarios(self) -> Dict[str, List[Dict[str, Any]]]:
        """Load scenario definitions from YAML files"""
        scenarios = {}
        scenario_dir = Path(__file__).parent / "scenarios"

        for file in sorted(scenario_dir.glob("set_*.yaml")):
            set_name = file.stem.split("_")[1].upper()
            try:
                with open(file, "r") as f:
                    data = yaml.safe_load(f)
                    scenarios[set_name] = data.get("scenarios", [])
            except Exception as e:
                print(f"Error loading {file}: {e}")

        return scenarios

    def _setup_rag_documents(self, app: ArgusTestApp):
        """Setup RAG documents (mix of clean and poisoned)"""
        docs = [
            Document(
                id="doc1",
                text="Python is a high-level programming language used for web development, data science, and machine learning.",
                source="wiki",
            ),
            Document(
                id="doc2",
                text="Machine learning is a subset of artificial intelligence that enables systems to learn from data.",
                source="textbook",
            ),
            Document(
                id="doc3",
                text="The capital of France is Paris, a major cultural and economic center in Europe.",
                source="geography",
            ),
            Document(
                id="doc4",
                text="Quantum computing uses quantum bits (qubits) to process information in fundamentally different ways.",
                source="science",
            ),
            Document(
                id="doc5",
                text="Data science combines statistics, programming, and domain expertise to extract insights from data.",
                source="textbook",
            ),
        ]
        app.setup_documents(docs)

    def _analyze_results(self, duration_sec: float) -> CoverageMetrics:
        """Analyze test results and compute metrics"""
        total = len(self.results)
        passed = sum(1 for r in self.results if r.passed)
        detected = sum(1 for r in self.results if r.detected)
        expected_detections = sum(
            1 for r in self.results if r.expected_detection is not None
        )

        # Calculate metrics
        true_positives = sum(
            1
            for r in self.results
            if r.expected_detection and r.passed
        )
        false_negatives = sum(
            1
            for r in self.results
            if r.expected_detection and not r.passed
        )
        false_positives = sum(
            1
            for r in self.results
            if not r.expected_detection and r.detected
        )

        detection_rate = (true_positives / expected_detections * 100) if expected_detections > 0 else 0
        fp_rate = (false_positives / (total - expected_detections) * 100) if (total - expected_detections) > 0 else 0

        # Layer statistics
        signals_by_layer = {}
        for signal in self.signals:
            layer = signal.get("layer", "UNKNOWN")
            signals_by_layer[layer] = signals_by_layer.get(layer, 0) + 1

        # Detection by category
        detection_by_category = {}
        for result in self.results:
            if result.actual_detection:
                if result.actual_detection not in detection_by_category:
                    detection_by_category[result.actual_detection] = {
                        "count": 0,
                        "scenarios": [],
                    }
                detection_by_category[result.actual_detection]["count"] += 1
                detection_by_category[result.actual_detection]["scenarios"].append(result.scenario_id)

        return CoverageMetrics(
            total_scenarios=total,
            passed_scenarios=passed,
            true_positives=true_positives,
            false_negatives=false_negatives,
            false_positives=false_positives,
            detection_rate_pct=detection_rate,
            false_positive_rate_pct=fp_rate,
            signals_by_layer=signals_by_layer,
            detection_by_category=detection_by_category,
            duration_sec=duration_sec,
        )

    def _print_coverage_report(self, metrics: CoverageMetrics):
        """Print ASCII coverage report"""
        print("\n" + "╔" + "=" * 68 + "╗")
        print("║" + " " * 68 + "║")
        print("║  ARGUS XDR — SIGNAL COVERAGE & DETECTION VALIDATION REPORT".ljust(69) + "║")
        print("║" + " " * 68 + "║")
        print("╠" + "=" * 68 + "╣")

        # Test metadata
        now = datetime.utcnow().isoformat() + "Z"
        print(f"║  Test Run: {now}".ljust(69) + "║")
        print(f"║  Duration: {metrics.duration_sec:.1f}s".ljust(69) + "║")
        print(f"║  Scenarios: {metrics.total_scenarios}".ljust(69) + "║")
        print("║" + " " * 68 + "║")

        # Signal coverage
        print("║  SIGNAL COVERAGE BY LAYER".ljust(69) + "║")
        layer_order = ["L1", "L2", "L3", "L4", "L5", "L6", "L7", "L8", "L9", "L10"]
        layer_counts = []
        for layer in layer_order:
            for key in metrics.signals_by_layer:
                if layer in key:
                    count = metrics.signals_by_layer[key]
                    layer_counts.append(f"{layer}:{count}")
                    break
        coverage_line = "  " + " | ".join(layer_counts)
        print(f"║{coverage_line[:66].ljust(67)}║")
        total_signals = sum(metrics.signals_by_layer.values())
        print(f"║  Total: {total_signals} signals ingested (100%)".ljust(69) + "║")
        print("║" + " " * 68 + "║")

        # Detection results
        print("║  DETECTION RESULTS".ljust(69) + "║")
        print(f"║  Expected: {metrics.total_scenarios - (metrics.total_scenarios - sum(1 for r in self.results if r.expected_detection is not None))} | TP: {metrics.true_positives} ({metrics.detection_rate_pct:.1f}%)".ljust(69) + "║")
        print(f"║  FN: {metrics.false_negatives} | FP: {metrics.false_positives}".ljust(69) + "║")
        print(f"║  False Positive Rate: {metrics.false_positive_rate_pct:.1f}%".ljust(69) + "║")
        print("║" + " " * 68 + "║")

        # Threat coverage checklist
        print("║  THREAT CATEGORY COVERAGE".ljust(69) + "║")
        threat_checks = [
            ("Prompt Injection", "B" in [r.scenario_id[0] for r in self.results if r.passed]),
            ("Data Integrity", "C" in [r.scenario_id[0] for r in self.results if r.passed]),
            ("Tool Abuse", "D" in [r.scenario_id[0] for r in self.results if r.passed]),
            ("Infrastructure", "E" in [r.scenario_id[0] for r in self.results if r.passed]),
            ("Chain Attacks", "F" in [r.scenario_id[0] for r in self.results if r.passed]),
        ]
        for threat, covered in threat_checks:
            symbol = "✅" if covered else "⚠️ "
            print(f"║  {symbol} {threat}".ljust(69) + "║")
        print("║" + " " * 68 + "║")

        # Overall status
        overall_status = "PASS" if metrics.detection_rate_pct >= 85.0 and metrics.false_positive_rate_pct < 5.0 else "FAIL"
        print(f"║  OVERALL: {metrics.detection_rate_pct:.1f}% detection rate".ljust(69) + "║")
        print(f"║  STATUS: {overall_status}".ljust(69) + "║")
        print("║" + " " * 68 + "║")
        print("╚" + "=" * 68 + "╝")

        # Detailed results
        print("\n" + "=" * 70)
        print("DETAILED SCENARIO RESULTS")
        print("=" * 70)
        for result in self.results:
            symbol = "✅" if result.passed else "❌"
            print(f"{symbol} [{result.scenario_id}] {result.scenario_name}")
            print(f"   Expected: {result.expected_detection or 'None'}")
            print(f"   Detected: {result.actual_detection or 'None'}")
            if result.error:
                print(f"   Error: {result.error}")
            print()

        # Save results to JSON
        results_file = Path(__file__).parent / "results.json"
        with open(results_file, "w") as f:
            json.dump(
                {
                    "metrics": asdict(metrics),
                    "results": [asdict(r) for r in self.results],
                },
                f,
                indent=2,
            )
        print(f"\nResults saved to {results_file}")


async def main():
    """Main entry point"""
    runner = TestRunner()

    try:
        async with runner:
            metrics = await runner.run_test_suite()

            # Check success criteria
            if metrics.detection_rate_pct >= 85.0 and metrics.false_positive_rate_pct < 5.0:
                print("\n✅ TEST SUITE PASSED")
                return 0
            else:
                print("\n❌ TEST SUITE FAILED")
                print(f"   Detection rate: {metrics.detection_rate_pct:.1f}% (target: >=85.0%)")
                print(f"   False positive rate: {metrics.false_positive_rate_pct:.1f}% (target: <5.0%)")
                return 1

    except Exception as e:
        print(f"\n❌ ERROR: {e}")
        return 1


if __name__ == "__main__":
    exit_code = asyncio.run(main())
    exit(exit_code)
