package trace

// SelectRunSignalsSQL fetches all signals for a trace_id, ordered by timestamp ASC.
// Column order is fixed and must match the Scan order in scanRunNode.
const SelectRunSignalsSQL = `
SELECT
    signal_id,
    span_id,
    parent_span_id,
    layer,
    category,
    severity,
    timestamp,
    duration_ms,
    enrich_baseline_deviation,
    ctx_l3_input_tokens,
    ctx_l3_output_tokens,
    ctx_l3_token_entropy,
    ctx_l4_kv_cache_hit_rate,
    ctx_l4_decode_time_ms,
    ctx_l5_temperature,
    ctx_l5_mean_logprob,
    ctx_l5_entropy_mean,
    ctx_l5_finish_reason,
    ctx_l6_safety_score,
    ctx_l6_jailbreak_score,
    ctx_l6_prompt_injection_score,
    ctx_l7_results_count,
    ctx_l7_latency_search_ms,
    ctx_l7_query_cache_hit,
    ctx_l8_tool_name,
    ctx_l8_tool_latency_ms,
    ctx_l8_step_number,
    ctx_l9_method,
    ctx_l9_path,
    ctx_l9_status_code,
    ctx_l9_latency_ms,
    ctx_l10_user_id,
    ctx_l10_session_id,
    ctx_l10_conversation_id,
    ctx_l10_event_type
FROM signals
WHERE trace_id = ?
ORDER BY timestamp ASC
`

// SelectSessionSignalsSQLBySessionID fetches all signals for a session_id, ordered by timestamp ASC.
// Column order matches SelectRunSignalsSQL plus session_id for cross-reference.
const SelectSessionSignalsSQLBySessionID = `
SELECT
    signal_id,
    span_id,
    parent_span_id,
    layer,
    category,
    severity,
    timestamp,
    duration_ms,
    enrich_baseline_deviation,
    ctx_l3_input_tokens,
    ctx_l3_output_tokens,
    ctx_l3_token_entropy,
    ctx_l4_kv_cache_hit_rate,
    ctx_l4_decode_time_ms,
    ctx_l5_temperature,
    ctx_l5_mean_logprob,
    ctx_l5_entropy_mean,
    ctx_l5_finish_reason,
    ctx_l6_safety_score,
    ctx_l6_jailbreak_score,
    ctx_l6_prompt_injection_score,
    ctx_l7_results_count,
    ctx_l7_latency_search_ms,
    ctx_l7_query_cache_hit,
    ctx_l8_tool_name,
    ctx_l8_tool_latency_ms,
    ctx_l8_step_number,
    ctx_l9_method,
    ctx_l9_path,
    ctx_l9_status_code,
    ctx_l9_latency_ms,
    ctx_l10_user_id,
    ctx_l10_session_id,
    ctx_l10_conversation_id,
    ctx_l10_event_type
FROM signals
WHERE session_id = ?
ORDER BY timestamp ASC
`

// SelectSessionSignalsSQLByConversationID fetches all signals for a conversation_id, ordered by timestamp ASC.
const SelectSessionSignalsSQLByConversationID = `
SELECT
    signal_id,
    span_id,
    parent_span_id,
    layer,
    category,
    severity,
    timestamp,
    duration_ms,
    enrich_baseline_deviation,
    ctx_l3_input_tokens,
    ctx_l3_output_tokens,
    ctx_l3_token_entropy,
    ctx_l4_kv_cache_hit_rate,
    ctx_l4_decode_time_ms,
    ctx_l5_temperature,
    ctx_l5_mean_logprob,
    ctx_l5_entropy_mean,
    ctx_l5_finish_reason,
    ctx_l6_safety_score,
    ctx_l6_jailbreak_score,
    ctx_l6_prompt_injection_score,
    ctx_l7_results_count,
    ctx_l7_latency_search_ms,
    ctx_l7_query_cache_hit,
    ctx_l8_tool_name,
    ctx_l8_tool_latency_ms,
    ctx_l8_step_number,
    ctx_l9_method,
    ctx_l9_path,
    ctx_l9_status_code,
    ctx_l9_latency_ms,
    ctx_l10_user_id,
    ctx_l10_session_id,
    ctx_l10_conversation_id,
    ctx_l10_event_type
FROM signals
WHERE conversation_id = ?
ORDER BY timestamp ASC
`
