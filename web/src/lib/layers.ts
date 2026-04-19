import { LAYERS, type Layer } from '../types'

export const LAYER_DESCRIPTIONS: Record<Layer, string> = {
  'L1_HARDWARE': 'Hardware (CPU, GPU, Memory)',
  'L2_MODEL_WEIGHTS': 'Model Weights & Quantization',
  'L3_TOKENIZER': 'Tokenization & Encoding',
  'L4_TRANSFORMER': 'Transformer Attention & KV Cache',
  'L5_OUTPUT_DECODING': 'Logits & Output Decoding',
  'L6_SAFETY': 'Safety Filters & Policy',
  'L7_RAG_RETRIEVAL': 'RAG & Vector Retrieval',
  'L8_AGENTS': 'Agent Tools & Orchestration',
  'L9_API_GATEWAY': 'API Gateway & Load Balancing',
  'L10_APPLICATION': 'Application & User Interface'
}

export const LAYER_COLORS: Record<Layer, string> = {
  'L1_HARDWARE': '#EF4444', // Red
  'L2_MODEL_WEIGHTS': '#F97316', // Orange
  'L3_TOKENIZER': '#EAB308', // Yellow
  'L4_TRANSFORMER': '#84CC16', // Lime
  'L5_OUTPUT_DECODING': '#22C55E', // Green
  'L6_SAFETY': '#10B981', // Emerald
  'L7_RAG_RETRIEVAL': '#06B6D4', // Cyan
  'L8_AGENTS': '#0EA5E9', // Sky
  'L9_API_GATEWAY': '#3B82F6', // Blue
  'L10_APPLICATION': '#F43F5E', // Rose
}

export function getLayerName(layerNum: number | string): Layer {
  const num = typeof layerNum === 'string' ? parseInt(layerNum) : layerNum
  return LAYERS[num - 1] || 'L10_APPLICATION'
}

export function getLayerDescription(layer: Layer): string {
  return LAYER_DESCRIPTIONS[layer] || 'Unknown Layer'
}

export function getLayerColor(layer: Layer): string {
  return LAYER_COLORS[layer] || '#666'
}
