import asyncio
from sdk.connector import Sensor, ConnectorType, Layer, Severity

async def test():
    # Use buffered connector for efficiency
    sensor = Sensor(
        connector_type=ConnectorType.BUFFER,
        config={
            "base_url": "http://localhost:8080",
            "app_id": "test-app",
            "max_batch_size": 10,
            "flush_interval_seconds": 2.0,
        }
    )

    print("Emitting test signal...")
    result = await sensor.emit(
        layer=Layer.L5_OUTPUT_DECODING,
        category="inference.test",
        severity=Severity.INFO,
        context={
            "model": "test",
            "input_tokens": 50,
            "output_tokens": 100,
        },
        duration_ms=125.5
    )

    print(f"Emission result: {result}")

    # Wait for auto-flush
    await asyncio.sleep(3)

    await sensor.close()
    print("Done!")

if __name__ == "__main__":
    asyncio.run(test())