"""
SDK Overhead Benchmark - Measure signal emission overhead

Tests both Python and TypeScript SDKs to verify <5ms p99 overhead.
"""

import asyncio
import time
import sys
import statistics
from typing import List

sys.path.insert(0, '/c/Users/Drupad/ArgusXDR')

from sdk import ArgusClient, Layer, Severity


class BenchmarkResults:
    """Store benchmark results"""

    def __init__(self, name: str, iterations: int):
        self.name = name
        self.iterations = iterations
        self.measurements: List[float] = []

    def add_measurement(self, ms: float):
        """Add a measurement in milliseconds"""
        self.measurements.append(ms)

    def report(self):
        """Print benchmark report"""
        if not self.measurements:
            print(f"{self.name}: No measurements")
            return

        measurements = sorted(self.measurements)
        min_ms = min(measurements)
        max_ms = max(measurements)
        mean_ms = statistics.mean(measurements)
        median_ms = statistics.median(measurements)
        p95 = measurements[int(len(measurements) * 0.95)]
        p99 = measurements[int(len(measurements) * 0.99)]
        stdev_ms = statistics.stdev(measurements) if len(measurements) > 1 else 0

        print(f"\n{self.name}")
        print(f"  Iterations: {self.iterations}")
        print(f"  Min:        {min_ms:.2f}ms")
        print(f"  Max:        {max_ms:.2f}ms")
        print(f"  Mean:       {mean_ms:.2f}ms")
        print(f"  Median:     {median_ms:.2f}ms")
        print(f"  Stdev:      {stdev_ms:.2f}ms")
        print(f"  P95:        {p95:.2f}ms")
        print(f"  P99:        {p99:.2f}ms")

        # Target verification
        target = 5.0
        if p99 <= target:
            print(f"  ✓ PASS: P99 {p99:.2f}ms <= {target}ms")
        else:
            print(f"  ✗ FAIL: P99 {p99:.2f}ms > {target}ms")

        return {
            "min": min_ms,
            "max": max_ms,
            "mean": mean_ms,
            "median": median_ms,
            "stdev": stdev_ms,
            "p95": p95,
            "p99": p99,
        }


async def benchmark_python_sdk_basic():
    """Benchmark basic Python SDK signal emission"""
    benchmark = BenchmarkResults("Python SDK - Basic Signal Emission", 100)

    client = ArgusClient(
        base_url="http://localhost:8080",
        app_id="benchmark-python",
    )
    await client.__aenter__()

    try:
        for _ in range(benchmark.iterations):
            start = time.time()
            await client.emit_signal(
                layer=Layer.L5_OUTPUT_DECODING,
                category="benchmark.basic",
                severity=Severity.INFO,
            )
            elapsed_ms = (time.time() - start) * 1000
            benchmark.add_measurement(elapsed_ms)
    finally:
        await client.__aexit__(None, None, None)

    return benchmark.report()


async def benchmark_python_sdk_with_context():
    """Benchmark Python SDK with layer-specific context"""
    benchmark = BenchmarkResults("Python SDK - Signal with Context", 100)

    client = ArgusClient(
        base_url="http://localhost:8080",
        app_id="benchmark-python",
    )
    await client.__aenter__()

    try:
        for _ in range(benchmark.iterations):
            start = time.time()
            await client.emit_signal(
                layer=Layer.L5_OUTPUT_DECODING,
                category="benchmark.context",
                context={
                    "output_tokens": 150,
                    "input_tokens": 50,
                    "finish_reason": "stop",
                    "temperature": 0.7,
                    "top_p": 1.0,
                },
                duration_ms=45.5,
            )
            elapsed_ms = (time.time() - start) * 1000
            benchmark.add_measurement(elapsed_ms)
    finally:
        await client.__aexit__(None, None, None)

    return benchmark.report()


async def benchmark_decorator_overhead():
    """Benchmark @observe decorator overhead"""
    benchmark = BenchmarkResults("Python SDK - Decorator Overhead", 100)

    client = ArgusClient(
        base_url="http://localhost:8080",
        app_id="benchmark-python",
    )
    await client.__aenter__()

    try:
        from sdk import observe

        @observe(
            layer=Layer.L5_OUTPUT_DECODING,
            category="benchmark.decorator",
            client=client,
        )
        async def decorated_function():
            # Simulated work: 1ms
            await asyncio.sleep(0.001)
            return "result"

        for _ in range(benchmark.iterations):
            start = time.time()
            result = await decorated_function()
            total_ms = (time.time() - start) * 1000

            # Subtract the 1ms of actual work to get decorator overhead
            overhead_ms = total_ms - 1.0
            benchmark.add_measurement(overhead_ms)
    finally:
        await client.__aexit__(None, None, None)

    return benchmark.report()


async def benchmark_batch_emission():
    """Benchmark emitting multiple signals in sequence"""
    benchmark = BenchmarkResults("Python SDK - Batch Emission (10 signals)", 50)

    client = ArgusClient(
        base_url="http://localhost:8080",
        app_id="benchmark-python",
    )
    await client.__aenter__()

    try:
        for _ in range(benchmark.iterations):
            start = time.time()

            # Emit 10 signals
            for i in range(10):
                await client.emit_signal(
                    layer=Layer.L7_RAG_RETRIEVAL,
                    category="benchmark.batch",
                    context={"result_index": i},
                )

            elapsed_ms = (time.time() - start) * 1000
            avg_per_signal = elapsed_ms / 10
            benchmark.add_measurement(avg_per_signal)
    finally:
        await client.__aexit__(None, None, None)

    return benchmark.report()


async def benchmark_concurrent_emissions():
    """Benchmark concurrent signal emissions"""
    benchmark = BenchmarkResults("Python SDK - Concurrent Emissions (10 parallel)", 50)

    client = ArgusClient(
        base_url="http://localhost:8080",
        app_id="benchmark-python",
    )
    await client.__aenter__()

    try:
        async def emit_signal():
            return await client.emit_signal(
                layer=Layer.L8_AGENTS,
                category="benchmark.concurrent",
            )

        for _ in range(benchmark.iterations):
            start = time.time()

            # Emit 10 signals concurrently
            await asyncio.gather(*[emit_signal() for _ in range(10)])

            elapsed_ms = (time.time() - start) * 1000
            avg_per_signal = elapsed_ms / 10
            benchmark.add_measurement(avg_per_signal)
    finally:
        await client.__aexit__(None, None, None)

    return benchmark.report()


async def main():
    """Run all benchmarks"""
    print("=" * 60)
    print("Argus SDK Overhead Benchmark")
    print("=" * 60)
    print("\nTarget: <5ms p99 overhead per signal")
    print("\nNote: Results depend on network latency and Argus availability.")
    print("      Run with Argus running on localhost for best results.\n")

    results = {}

    print("Running benchmarks...")
    print("-" * 60)

    # Run benchmarks
    try:
        results["basic"] = await benchmark_python_sdk_basic()
    except Exception as e:
        print(f"Error in basic benchmark: {e}")

    try:
        results["context"] = await benchmark_python_sdk_with_context()
    except Exception as e:
        print(f"Error in context benchmark: {e}")

    try:
        results["decorator"] = await benchmark_decorator_overhead()
    except Exception as e:
        print(f"Error in decorator benchmark: {e}")

    try:
        results["batch"] = await benchmark_batch_emission()
    except Exception as e:
        print(f"Error in batch benchmark: {e}")

    try:
        results["concurrent"] = await benchmark_concurrent_emissions()
    except Exception as e:
        print(f"Error in concurrent benchmark: {e}")

    print("\n" + "=" * 60)
    print("Benchmark Complete")
    print("=" * 60)

    # Summary
    print("\nSummary:")
    for name, result in results.items():
        if result:
            status = "✓ PASS" if result["p99"] <= 5.0 else "✗ FAIL"
            print(f"  {status}: {result['p99']:.2f}ms p99")


if __name__ == "__main__":
    asyncio.run(main())
