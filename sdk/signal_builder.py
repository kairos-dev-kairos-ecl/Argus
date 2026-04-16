"""
SignalBuilder — Construct ArgusSignal messages from context

This module provides utilities for building ArgusSignal protobuf messages
from application context, metrics, and span data.
"""

import uuid
from datetime import datetime, timezone
from typing import Any, Dict, Optional

from google.protobuf.timestamp_pb2 import Timestamp
from .gen.argus.v1 import signal_pb2


class SignalBuilder:
    """Build ArgusSignal messages from context"""

    @staticmethod
    def build(
        layer: int,
        category: str,
        app_id: str,
        app_version: str,
        sdk_version: str,
        environment: str,
        instance_id: str,
        severity: int = 1,
        duration_ms: Optional[float] = None,
        trace_id: Optional[str] = None,
        span_id: Optional[str] = None,
        parent_span_id: Optional[str] = None,
        context: Optional[Dict[str, Any]] = None,
    ) -> signal_pb2.ArgusSignal:
        """
        Build an ArgusSignal message.

        Args:
            layer: Layer enum value (1-10)
            category: Signal category (e.g., "retrieval.search")
            app_id: Application identifier
            app_version: Application version
            sdk_version: SDK version
            environment: Environment (dev|staging|prod)
            instance_id: Instance identifier
            severity: Severity level (1-5)
            duration_ms: Operation duration
            trace_id: Distributed trace ID
            span_id: Span ID
            parent_span_id: Parent span ID
            context: Layer-specific context

        Returns:
            ArgusSignal protobuf message
        """
        signal_id = SignalBuilder._ulid()
        trace_id = trace_id or str(uuid.uuid4())
        span_id = span_id or str(uuid.uuid4())[:8]

        now = datetime.now(timezone.utc)
        timestamp = Timestamp()
        timestamp.FromDatetime(now)

        signal = signal_pb2.ArgusSignal(
            signal_id=signal_id,
            trace_id=trace_id,
            span_id=span_id,
            parent_span_id=parent_span_id or "",
            layer=layer,
            category=category,
            severity=severity,
            timestamp=timestamp,
            duration_ms=duration_ms or 0.0,
        )

        # Add source
        source = signal_pb2.Source(
            app_id=app_id,
            app_version=app_version,
            sdk_version=sdk_version,
            environment=environment,
            instance_id=instance_id,
        )
        signal.source.CopyFrom(source)

        # Add context if provided
        if context:
            SignalBuilder._set_context(signal, layer, context)

        return signal

    @staticmethod
    def _set_context(
        signal: signal_pb2.ArgusSignal, layer: int, context: Dict[str, Any]
    ):
        """Set layer-specific context on signal"""
        if layer == 1:  # L1_HARDWARE
            ctx = signal_pb2.ContextL1()
            for field in [
                "cpu_usage_pct", "memory_used_mb", "memory_total_mb",
                "gpu_utilization_pct", "gpu_memory_used_mb", "gpu_memory_total_mb",
                "gpu_temperature_c", "gpu_power_draw_w", "gpu_count", "gpu_model",
                "disk_read_mbps", "disk_write_mbps", "net_rx_mbps", "net_tx_mbps",
                "cpu_steal_pct", "load_avg_1m", "load_avg_5m", "load_avg_15m",
                "open_file_descriptors", "thread_count", "host_id", "instance_type",
                "process_cpu_pct", "process_memory_mb",
            ]:
                if field in context:
                    setattr(ctx, field, context[field])
            signal.context_l1.CopyFrom(ctx)

        elif layer == 2:  # L2_MODEL_WEIGHTS
            ctx = signal_pb2.ContextL2()
            for field in [
                "model_id", "model_family", "model_version", "model_hash",
                "quantization", "parameter_count", "model_size_gb", "load_time_ms",
                "vram_allocated_gb", "ram_allocated_gb", "precision", "backend",
                "adapter_id", "adapter_hash", "weights_verified",
                "weights_load_latency_ms", "layers_offloaded", "device_map",
            ]:
                if field in context:
                    setattr(ctx, field, context[field])
            signal.context_l2.CopyFrom(ctx)

        elif layer == 3:  # L3_TOKENIZER
            ctx = signal_pb2.ContextL3()
            for field in [
                "input_tokens", "output_tokens", "total_tokens", "tokenizer_id",
                "tokenizer_version", "encoding_time_ms", "decoding_time_ms",
                "context_window_size", "context_utilization_pct", "truncated_tokens",
                "truncated", "chat_template", "bos_token", "avg_token_length",
                "unique_token_count", "token_entropy", "special_token_count",
                "encoding_name", "vocab_size", "compression_ratio", "oov_rate",
                "has_special_tokens",
            ]:
                if field in context:
                    setattr(ctx, field, context[field])
            signal.context_l3.CopyFrom(ctx)

        elif layer == 4:  # L4_TRANSFORMER
            ctx = signal_pb2.ContextL4()
            for field in [
                "kv_cache_used_mb", "kv_cache_total_mb", "kv_cache_hit_rate",
                "num_heads", "num_layers", "attention_entropy_mean",
                "attention_entropy_max", "attention_entropy_min",
                "prefill_time_ms", "decode_time_ms", "batch_size", "sequence_length",
                "throughput_tokens_per_sec", "memory_bandwidth_gbps",
                "flash_attention", "paged_attention", "speculative_tokens",
                "speculation_accept_rate", "context_length_used", "active_sequences",
                "flops_utilized", "attention_implementation", "compute_dtype",
            ]:
                if field in context:
                    setattr(ctx, field, context[field])
            signal.context_l4.CopyFrom(ctx)

        elif layer == 5:  # L5_OUTPUT_DECODING
            ctx = signal_pb2.ContextL5(
                operation=context.get("operation", 0),
                output_tokens=context.get("output_tokens", 0),
                input_tokens=context.get("input_tokens", 0),
                total_tokens=context.get("total_tokens", 0),
                finish_reason=context.get("finish_reason", ""),
                temperature=context.get("temperature", 0.7),
                top_p=context.get("top_p", 1.0),
                ttft_ms=context.get("ttft_ms", 0.0),
                tps=context.get("tps", 0.0),
                stop_sequences=context.get("stop_sequences", []),
            )
            for field in [
                "mean_logprob", "min_logprob", "entropy_mean", "entropy_variance",
                "top_k", "min_p", "repetition_penalty", "presence_penalty",
                "frequency_penalty", "seed", "output_repetition_score",
                "distinct_ngrams_ratio", "truncated", "finish_reason_detail",
            ]:
                if field in context:
                    setattr(ctx, field, context[field])
            signal.context_l5.CopyFrom(ctx)

        elif layer == 6:  # L6_SAFETY
            ctx = signal_pb2.ContextL6()
            for field in [
                "safety_score", "safe", "action_taken", "classifier_id",
                "toxicity_score", "bias_score", "policy_id", "policy_version",
                "policy_violated", "violation_type", "redaction_method",
                "redacted_tokens", "evaluation_time_ms", "input_language",
                "language_confidence", "jailbreak_detected", "jailbreak_score",
                "prompt_injection_detected", "prompt_injection_score",
                "pii_types_detected", "pii_redacted",
            ]:
                if field in context:
                    setattr(ctx, field, context[field])
            # Handle repeated SafetyCategoryScore messages
            if "category_scores" in context:
                for score_dict in context["category_scores"]:
                    score = signal_pb2.SafetyCategoryScore(
                        category=score_dict.get("category", ""),
                        score=score_dict.get("score", 0.0),
                        flagged=score_dict.get("flagged", False),
                    )
                    ctx.category_scores.append(score)
            signal.context_l6.CopyFrom(ctx)

        elif layer == 7:  # L7_RAG_RETRIEVAL
            ctx = signal_pb2.ContextL7(
                operation=context.get("operation", 0),
                query_text=context.get("query_text", ""),
                results_count=context.get("results_count", 0),
                embedding_model=context.get("embedding_model", ""),
                vector_index=context.get("vector_index", ""),
                context_window_pct=context.get("context_window_pct", 0.0),
                reranker_applied=context.get("reranker_applied", False),
                query_embedding=context.get("query_embedding", []),
                results_scores=context.get("results_scores", []),
                top_chunk_source=context.get("top_chunk_source", ""),
            )
            for field in [
                "reranker_delta", "query_cache_hit", "query_cache_key",
                "citations_used", "citation_overlap_score", "index_name",
                "index_type", "index_document_count", "index_freshness_hours",
                "reranker_score", "reranker_model", "hybrid_search",
                "sparse_weight", "dense_weight",
            ]:
                if field in context:
                    setattr(ctx, field, context[field])
            if "latency_breakdown" in context:
                lb = context["latency_breakdown"]
                ctx.latency_breakdown.CopyFrom(signal_pb2.LatencyBreakdown(
                    embedding_ms=lb.get("embedding_ms", 0.0),
                    search_ms=lb.get("search_ms", 0.0),
                    rerank_ms=lb.get("rerank_ms", 0.0),
                ))
            signal.context_l7.CopyFrom(ctx)

        elif layer == 8:  # L8_AGENTS
            ctx = signal_pb2.ContextL8(
                operation=context.get("operation", 0),
                tool_name=context.get("tool_name", ""),
                tool_provider=context.get("tool_provider", ""),
                tool_result=context.get("tool_result", ""),
                tool_latency_ms=context.get("tool_latency_ms", 0.0),
                step_number=context.get("step_number", 0),
                data_flow_tags=context.get("data_flow_tags", []),
                permissions_used=context.get("permissions_used", []),
                permissions_requested=context.get("permissions_requested", []),
                capabilities_used=context.get("capabilities_used", []),
            )
            for field in [
                "tool_error", "total_steps", "task_id", "parent_task_id",
                "task_depth", "subtask_count", "memory_operation",
                "memory_items_read", "memory_items_written", "code_language",
                "code_executed", "code_exit_code", "code_execution_time_ms",
                "sandboxed", "orchestrator_type",
            ]:
                if field in context:
                    setattr(ctx, field, context[field])
            if "tool_arguments" in context:
                ctx.tool_arguments.update(context["tool_arguments"])
            signal.context_l8.CopyFrom(ctx)

        elif layer == 9:  # L9_API_GATEWAY
            ctx = signal_pb2.ContextL9()
            for field in [
                "method", "path", "status_code", "latency_ms", "client_ip",
                "user_agent", "api_key_id", "api_version", "request_size_bytes",
                "response_size_bytes", "cache_hit", "cache_key",
                "rate_limit_remaining", "rate_limit_reset_ms", "rate_limited",
                "auth_method", "authenticated", "client_id", "request_id",
                "upstream_id", "upstream_latency_ms", "ssl_terminated",
                "protocol", "region", "endpoint_alias",
            ]:
                if field in context:
                    setattr(ctx, field, context[field])
            signal.context_l9.CopyFrom(ctx)

        elif layer == 10:  # L10_APPLICATION
            ctx = signal_pb2.ContextL10()
            for field in [
                "user_id", "session_id", "conversation_id", "event_type",
                "component", "action", "feature_id", "ab_variant",
                "request_retry_count", "client_latency_ms", "error_code",
                "error_message", "error_recovered", "platform", "app_version",
                "locale", "timezone", "page_id", "referrer",
                "satisfaction_score", "feedback_provided", "deployment_id",
            ]:
                if field in context:
                    setattr(ctx, field, context[field])
            if "feature_flags" in context:
                ctx.feature_flags.update(context["feature_flags"])
            if "custom_attributes" in context:
                ctx.custom_attributes.update(context["custom_attributes"])
            signal.context_l10.CopyFrom(ctx)

        elif layer == 11:  # L_DECISION
            ctx = signal_pb2.ContextLDecision(
                decision=context.get("decision", 0),
                confidence=context.get("confidence", 0.0),
                reasoning=context.get("reasoning", ""),
                recommended_action=context.get("recommended_action", 0),
                policy_version=context.get("policy_version", ""),
                policy_name=context.get("policy_name", ""),
                evaluation_time_ms=context.get("evaluation_time_ms", 0.0),
                matched_indicators=context.get("matched_indicators", []),
            )
            for field in [
                "policy_id", "triggering_rule_id", "triggering_signal_id",
                "failed_open", "decision_trace_id", "alert_threshold",
            ]:
                if field in context:
                    setattr(ctx, field, context[field])
            signal.context_l_decision.CopyFrom(ctx)

    @staticmethod
    def _ulid() -> str:
        """Generate a ULID-like unique ID"""
        import time
        timestamp_ms = int(time.time() * 1000)
        random_part = uuid.uuid4().hex[:12].upper()
        return f"{timestamp_ms:010d}{random_part}"
