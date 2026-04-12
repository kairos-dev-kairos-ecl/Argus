import React, { useState } from 'react'
import { colors } from '../lib/design-tokens'

/**
 * Token with confidence metadata
 */
export interface ConfidenceToken {
  text: string
  logprob: number // Log probability (-5 to 0, higher is more confident)
  rank: number // Token rank in model output
  alternatives?: Array<{
    text: string
    logprob: number
  }>
}

interface TokenConfidenceViewProps {
  tokens: ConfidenceToken[]
  onTokenHover?: (token: ConfidenceToken | null) => void
}

/**
 * Maps logprob to color: high (green) -> medium (yellow) -> low (red)
 * logprob range: -0.1 (high) to -5 (low)
 */
function getConfidenceColor(logprob: number): string {
  // Normalize: -0.1 -> 1.0 (high), -5 -> 0 (low)
  const normalized = Math.max(0, Math.min(1, (logprob + 0.1) / -5.1 + 1))

  if (normalized > 0.7) {
    // High confidence: green
    return colors.status.success
  } else if (normalized > 0.4) {
    // Medium confidence: yellow
    return colors.status.warning
  } else {
    // Low confidence: red
    return colors.status.error
  }
}

/**
 * Computes confidence statistics for a token stream
 */
function computeStats(tokens: ConfidenceToken[]) {
  if (tokens.length === 0) {
    return {
      meanConfidence: 0,
      minLogprob: 0,
      entropy: 0,
      hallucination_risk: 0,
    }
  }

  const logprobs = tokens.map((t) => t.logprob)
  const minLogprob = Math.min(...logprobs)

  // Normalize to [0, 1]: -0.1 = 1.0, -5 = 0
  const normalized = logprobs.map((p) => Math.max(0, Math.min(1, (p + 0.1) / -5.1 + 1)))
  const meanConfidence = normalized.reduce((a, b) => a + b, 0) / normalized.length

  // Simple entropy estimate: if many low-confidence tokens, higher entropy
  const lowConfCount = normalized.filter((c) => c < 0.4).length
  const entropy = lowConfCount / normalized.length

  // Hallucination risk: mean confidence < 0.5 OR entropy > 0.3
  const hallucination_risk = meanConfidence < 0.5 || entropy > 0.3

  return {
    meanConfidence: meanConfidence * 100, // Percentage
    minLogprob,
    entropy: entropy * 100, // Percentage of low-confidence tokens
    hallucination_risk,
  }
}

/**
 * TokenConfidenceView: Displays tokens with confidence-based coloring and hover details
 */
export const TokenConfidenceView: React.FC<TokenConfidenceViewProps> = ({
  tokens,
  onTokenHover,
}) => {
  const [hoveredToken, setHoveredToken] = useState<ConfidenceToken | null>(null)
  const stats = computeStats(tokens)

  const handleTokenMouseEnter = (token: ConfidenceToken) => {
    setHoveredToken(token)
    onTokenHover?.(token)
  }

  const handleTokenMouseLeave = () => {
    setHoveredToken(null)
    onTokenHover?.(null)
  }

  return (
    <div className="space-y-4">
      {/* Statistics */}
      <div className="grid grid-cols-2 gap-3 bg-muted-background p-3 rounded-md">
        <div>
          <div className="text-xs text-muted-foreground">Mean Confidence</div>
          <div className="text-lg font-semibold text-foreground">
            {stats.meanConfidence.toFixed(1)}%
          </div>
        </div>
        <div>
          <div className="text-xs text-muted-foreground">Low-Conf Tokens</div>
          <div className="text-lg font-semibold text-foreground">{stats.entropy.toFixed(1)}%</div>
        </div>
        <div>
          <div className="text-xs text-muted-foreground">Min Logprob</div>
          <div className="text-lg font-semibold text-foreground font-mono">
            {stats.minLogprob.toFixed(2)}
          </div>
        </div>
        <div>
          <div className="text-xs text-muted-foreground">Hallucination Risk</div>
          <div
            className={`text-lg font-semibold ${
              stats.hallucination_risk
                ? 'text-status-error'
                : 'text-status-success'
            }`}
          >
            {stats.hallucination_risk ? 'HIGH' : 'LOW'}
          </div>
        </div>
      </div>

      {/* Token stream */}
      <div className="bg-muted-background p-3 rounded-md">
        <div className="text-xs font-semibold text-muted-foreground mb-2">Token Confidence Heatmap</div>
        <div className="flex flex-wrap gap-1">
          {tokens.map((token, idx) => {
            const isHovered = hoveredToken === token
            const bgColor = getConfidenceColor(token.logprob)

            return (
              <div key={idx} className="relative group">
                <div
                  style={{
                    backgroundColor: bgColor,
                    opacity: isHovered ? 1 : 0.7,
                    transition: 'opacity 150ms ease-out',
                  }}
                  className="px-2 py-1 rounded text-sm font-mono text-white cursor-pointer hover:opacity-100"
                  onMouseEnter={() => handleTokenMouseEnter(token)}
                  onMouseLeave={handleTokenMouseLeave}
                >
                  {token.text}
                </div>

                {/* Tooltip on hover */}
                {isHovered && (
                  <div
                    className="absolute z-50 bg-background border border-border rounded-md p-2 shadow-lg whitespace-nowrap text-xs"
                    style={{
                      bottom: '100%',
                      left: '50%',
                      transform: 'translateX(-50%)',
                      marginBottom: '4px',
                    }}
                  >
                    <div className="text-foreground font-mono mb-1">{token.text}</div>
                    <div className="text-muted-foreground">
                      logprob: <span className="text-foreground font-semibold">{token.logprob.toFixed(3)}</span>
                    </div>
                    <div className="text-muted-foreground">
                      rank: <span className="text-foreground font-semibold">{token.rank}</span>
                    </div>

                    {token.alternatives && token.alternatives.length > 0 && (
                      <div className="mt-2 pt-2 border-t border-border">
                        <div className="text-muted-foreground text-xs mb-1">Top Alternatives:</div>
                        {token.alternatives.slice(0, 3).map((alt, altIdx) => (
                          <div key={altIdx} className="text-muted-foreground text-xs">
                            "{alt.text}" (
                            <span className="text-foreground font-semibold">{alt.logprob.toFixed(3)}</span>)
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </div>

      {/* Legend */}
      <div className="text-xs text-muted-foreground space-y-1">
        <div className="flex items-center gap-2">
          <div
            className="w-4 h-4 rounded"
            style={{ backgroundColor: colors.status.success }}
          />
          <span>High confidence (logprob &gt; -0.1)</span>
        </div>
        <div className="flex items-center gap-2">
          <div
            className="w-4 h-4 rounded"
            style={{ backgroundColor: colors.status.warning }}
          />
          <span>Medium confidence (-0.1 to -0.5)</span>
        </div>
        <div className="flex items-center gap-2">
          <div
            className="w-4 h-4 rounded"
            style={{ backgroundColor: colors.status.error }}
          />
          <span>Low confidence (&lt; -0.5)</span>
        </div>
      </div>
    </div>
  )
}
