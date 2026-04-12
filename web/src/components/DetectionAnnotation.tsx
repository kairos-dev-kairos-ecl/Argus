import React from 'react'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import type { Detection } from '../types/index'

dayjs.extend(relativeTime)

interface DetectionAnnotationProps {
  detection: Detection
}

/**
 * Component displaying a single detection result annotation.
 * Shows rule name, severity, confidence, and when the detection matched.
 */
export const DetectionAnnotation: React.FC<DetectionAnnotationProps> = ({ detection }) => {
  const severityColor = {
    1: 'bg-blue-500',
    2: 'bg-status-success500',
    3: 'bg-yellow-500',
    4: 'bg-orange-500',
    5: 'bg-status-error500'
  }[detection.severity] || 'bg-slate-500'

  const severityTextColor = {
    1: 'text-blue-200',
    2: 'text-status-success200',
    3: 'text-yellow-200',
    4: 'text-orange-200',
    5: 'text-status-error200'
  }[detection.severity] || 'text-slate-200'

  return (
    <div className="p-3 bg-background rounded border border-border hover:border-slate-600 transition-colors">
      <div className="flex items-center justify-between mb-2">
        <span className="font-semibold text-foreground text-sm">{detection.rule_name}</span>
        <div className={`${severityColor} ${severityTextColor} px-2 py-1 rounded text-xs font-bold`}>
          Sev {detection.severity}
        </div>
      </div>
      <div className="text-xs text-slate-400 flex gap-3">
        <span>Confidence: {(detection.confidence * 100).toFixed(0)}%</span>
        <span className="text-slate-500">{dayjs(detection.matched_at).fromNow()}</span>
      </div>
    </div>
  )
}
