import { useState } from "react";
import { Check, Copy, AlertTriangle } from "lucide-react";

type Step = "org" | "ingestion" | "token" | "validation";

interface OnboardingState {
  orgName: string;
  ingestionMethod: string;
  apiToken: string;
  validationStatus: "idle" | "validating" | "success" | "error";
}

export function OnboardingFlow() {
  const [currentStep, setCurrentStep] = useState<Step>("org");
  const [state, setState] = useState<OnboardingState>({
    orgName: "",
    ingestionMethod: "",
    apiToken: "",
    validationStatus: "idle",
  });
  const [tokenCopied, setTokenCopied] = useState(false);
  const [tokenRevealed, setTokenRevealed] = useState(false);
  
  const steps: { id: Step; label: string; index: number }[] = [
    { id: "org", label: "ORG SETUP", index: 1 },
    { id: "ingestion", label: "INGESTION METHOD", index: 2 },
    { id: "token", label: "TOKEN VAULTING", index: 3 },
    { id: "validation", label: "VALIDATION", index: 4 },
  ];
  
  const currentStepIndex = steps.findIndex(s => s.id === currentStep);
  
  const handleNext = () => {
    if (currentStep === "org" && state.orgName) {
      setCurrentStep("ingestion");
    } else if (currentStep === "ingestion" && state.ingestionMethod) {
      // Generate mock token
      const mockToken = `xdr_${Math.random().toString(36).substring(2, 15)}_${Math.random().toString(36).substring(2, 15)}`;
      setState({ ...state, apiToken: mockToken });
      setCurrentStep("token");
    } else if (currentStep === "token" && state.apiToken) {
      setCurrentStep("validation");
      // Simulate validation
      setState({ ...state, validationStatus: "validating" });
      setTimeout(() => {
        setState(prev => ({ ...prev, validationStatus: "success" }));
      }, 2000);
    }
  };
  
  const copyToken = () => {
    navigator.clipboard.writeText(state.apiToken);
    setTokenCopied(true);
    setTimeout(() => setTokenCopied(false), 2000);
  };
  
  return (
    <div className="h-full flex items-center justify-center p-8" style={{ 
      background: 'var(--color-background)',
      fontFamily: 'var(--font-primary)'
    }}>
      <div className="w-full max-w-4xl" style={{
        background: 'var(--color-surface)',
        border: 'var(--border-stark)',
        minHeight: '600px'
      }}>
        <div className="flex h-full">
          {/* Progress Tracker - 30% */}
          <div className="w-[30%] p-8" style={{ borderRight: 'var(--border-stark)' }}>
            <div className="space-y-6">
              {steps.map((step, idx) => {
                const isCompleted = idx < currentStepIndex;
                const isCurrent = step.id === currentStep;
                
                return (
                  <div key={step.id} className="flex items-start gap-3">
                    {/* Step Indicator */}
                    <div 
                      className="flex items-center justify-center shrink-0"
                      style={{
                        width: '24px',
                        height: '24px',
                        background: isCurrent ? 'var(--color-primary)' : 'var(--color-surface)',
                        color: isCurrent ? '#050506' : 'var(--color-text)',
                        border: '1px solid',
                        borderColor: isCompleted ? 'var(--color-primary)' : 'var(--color-muted)',
                        fontSize: '11px',
                        fontWeight: 600
                      }}
                    >
                      {isCompleted ? <Check size={14} /> : step.index}
                    </div>
                    
                    {/* Step Label */}
                    <div>
                      <div style={{
                        fontFamily: 'var(--font-display)',
                        fontSize: '12px',
                        fontWeight: 600,
                        color: isCurrent ? 'var(--color-primary)' : 'var(--color-text)',
                        letterSpacing: '1px'
                      }}>
                        {step.label}
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
          
          {/* Content Area - 70% */}
          <div className="flex-1 p-8 flex flex-col">
            {/* Step Content */}
            <div className="flex-1">
              {currentStep === "org" && (
                <div>
                  <h2 style={{
                    fontFamily: 'var(--font-display)',
                    fontSize: '20px',
                    fontWeight: 700,
                    color: 'var(--color-text)',
                    marginBottom: '8px'
                  }}>
                    ORGANIZATION SETUP
                  </h2>
                  <p style={{ fontSize: '11px', color: 'var(--color-muted)', marginBottom: '24px' }}>
                    Initialize your zero-trust organizational context
                  </p>
                  
                  <div>
                    <label style={{ 
                      display: 'block',
                      fontSize: '11px', 
                      color: 'var(--color-text)',
                      marginBottom: '8px',
                      letterSpacing: '1px'
                    }}>
                      ORG NAME
                    </label>
                    <input
                      type="text"
                      value={state.orgName}
                      onChange={(e) => setState({ ...state, orgName: e.target.value })}
                      placeholder="Enter organization name_"
                      className="w-full px-4 py-3"
                      style={{
                        background: '#050506',
                        border: '1px solid var(--color-muted)',
                        color: 'var(--color-text)',
                        fontSize: '13px',
                        outline: 'none'
                      }}
                    />
                  </div>
                </div>
              )}
              
              {currentStep === "ingestion" && (
                <div>
                  <h2 style={{
                    fontFamily: 'var(--font-display)',
                    fontSize: '20px',
                    fontWeight: 700,
                    color: 'var(--color-text)',
                    marginBottom: '8px'
                  }}>
                    INGESTION METHOD
                  </h2>
                  <p style={{ fontSize: '11px', color: 'var(--color-muted)', marginBottom: '24px' }}>
                    Select telemetry ingestion pipeline
                  </p>
                  
                  <div className="space-y-3">
                    {["OpenTelemetry Agent", "Direct API Push", "Kafka Stream", "S3 Batch Import"].map((method) => (
                      <button
                        key={method}
                        onClick={() => setState({ ...state, ingestionMethod: method })}
                        className="w-full px-4 py-3 text-left transition-colors"
                        style={{
                          background: state.ingestionMethod === method ? 'rgba(0, 240, 255, 0.1)' : 'transparent',
                          border: '1px solid',
                          borderColor: state.ingestionMethod === method ? 'var(--color-primary)' : 'var(--color-muted)',
                          color: state.ingestionMethod === method ? 'var(--color-primary)' : 'var(--color-text)',
                          fontSize: '13px'
                        }}
                      >
                        {method}
                      </button>
                    ))}
                  </div>
                </div>
              )}
              
              {currentStep === "token" && (
                <div>
                  <h2 style={{
                    fontFamily: 'var(--font-display)',
                    fontSize: '20px',
                    fontWeight: 700,
                    color: 'var(--color-text)',
                    marginBottom: '8px'
                  }}>
                    TOKEN VAULTING
                  </h2>
                  <p style={{ fontSize: '11px', color: 'var(--color-muted)', marginBottom: '24px' }}>
                    Secure API credentials for zero-trust access
                  </p>
                  
                  <div className="p-4 mb-4" style={{ 
                    background: 'rgba(255, 179, 0, 0.1)',
                    border: '1px solid var(--color-warning)'
                  }}>
                    <div className="flex gap-2">
                      <AlertTriangle size={16} style={{ color: 'var(--color-warning)' }} />
                      <p style={{ fontSize: '11px', color: 'var(--color-warning)' }}>
                        ONE-TIME DISPLAY: This token will only be shown once. Copy and secure it immediately.
                      </p>
                    </div>
                  </div>
                  
                  <div>
                    <label style={{ 
                      display: 'block',
                      fontSize: '11px', 
                      color: 'var(--color-text)',
                      marginBottom: '8px',
                      letterSpacing: '1px'
                    }}>
                      API TOKEN
                    </label>
                    <div className="flex gap-2">
                      <input
                        type={tokenRevealed ? "text" : "password"}
                        value={state.apiToken}
                        readOnly
                        className="flex-1 px-4 py-3"
                        style={{
                          background: '#050506',
                          border: tokenCopied ? '1px solid var(--color-primary)' : '1px solid var(--color-muted)',
                          color: 'var(--color-primary)',
                          fontSize: '13px',
                          outline: 'none'
                        }}
                      />
                      <button
                        onClick={() => setTokenRevealed(!tokenRevealed)}
                        className="px-4 py-3"
                        style={{
                          background: 'transparent',
                          border: '1px solid var(--color-muted)',
                          color: 'var(--color-text)',
                          fontSize: '11px',
                          cursor: 'pointer'
                        }}
                      >
                        {tokenRevealed ? "HIDE" : "SHOW"}
                      </button>
                      <button
                        onClick={copyToken}
                        className="px-4 py-3 flex items-center gap-2"
                        style={{
                          background: tokenCopied ? 'var(--color-primary)' : 'transparent',
                          border: '1px solid',
                          borderColor: tokenCopied ? 'var(--color-primary)' : 'var(--color-muted)',
                          color: tokenCopied ? '#050506' : 'var(--color-text)',
                          fontSize: '11px',
                          cursor: 'pointer',
                          fontFamily: 'var(--font-display)',
                          fontWeight: 600,
                          letterSpacing: '1px'
                        }}
                      >
                        <Copy size={14} />
                        {tokenCopied ? "COPIED" : "COPY"}
                      </button>
                    </div>
                  </div>
                </div>
              )}
              
              {currentStep === "validation" && (
                <div>
                  <h2 style={{
                    fontFamily: 'var(--font-display)',
                    fontSize: '20px',
                    fontWeight: 700,
                    color: 'var(--color-text)',
                    marginBottom: '8px'
                  }}>
                    CONNECTION VALIDATION
                  </h2>
                  <p style={{ fontSize: '11px', color: 'var(--color-muted)', marginBottom: '24px' }}>
                    Verifying telemetry ingestion pipeline
                  </p>
                  
                  {/* Validation Console */}
                  <div className="p-4" style={{
                    background: '#050506',
                    border: '1px solid var(--color-muted)',
                    height: '300px',
                    overflowY: 'auto'
                  }}>
                    <div style={{ fontSize: '11px', color: 'var(--color-text)' }}>
                      <div style={{ color: 'var(--color-muted)' }}>[2026-04-16 14:32:01 UTC]</div>
                      <div className="mt-2">→ Initiating handshake with cluster-01.us-west...</div>
                      <div className="mt-1" style={{ color: 'var(--color-primary)' }}>✓ Connection established</div>
                      
                      {state.validationStatus !== "idle" && (
                        <>
                          <div className="mt-4" style={{ color: 'var(--color-muted)' }}>[2026-04-16 14:32:03 UTC]</div>
                          <div className="mt-2">→ Validating L1-L4 telemetry streams...</div>
                          
                          {state.validationStatus === "validating" && (
                            <div className="mt-2" style={{ color: 'var(--color-warning)' }}>
                              ⟳ Waiting for initial telemetry ping...
                            </div>
                          )}
                          
                          {state.validationStatus === "success" && (
                            <>
                              <div className="mt-2" style={{ color: 'var(--color-primary)' }}>✓ L1 Hardware telemetry: ACTIVE</div>
                              <div className="mt-1" style={{ color: 'var(--color-primary)' }}>✓ L2 Network telemetry: ACTIVE</div>
                              <div className="mt-1" style={{ color: 'var(--color-primary)' }}>✓ L3 Infrastructure: ACTIVE</div>
                              <div className="mt-1" style={{ color: 'var(--color-primary)' }}>✓ L4 Data Pipeline: ACTIVE</div>
                              
                              <div className="mt-4 p-3" style={{ 
                                background: 'rgba(0, 240, 255, 0.1)',
                                border: '1px solid var(--color-primary)'
                              }}>
                                <div style={{ color: 'var(--color-primary)', fontWeight: 600 }}>
                                  ✓ VALIDATION COMPLETE
                                </div>
                                <div className="mt-1" style={{ fontSize: '10px' }}>
                                  All telemetry layers operational. System ready for observability.
                                </div>
                              </div>
                            </>
                          )}
                        </>
                      )}
                    </div>
                  </div>
                </div>
              )}
            </div>
            
            {/* Actions */}
            <div className="flex justify-end gap-3 mt-6 pt-6" style={{ borderTop: 'var(--border-stark)' }}>
              {currentStepIndex > 0 && currentStep !== "validation" && (
                <button
                  onClick={() => setCurrentStep(steps[currentStepIndex - 1].id)}
                  className="px-6 py-3"
                  style={{
                    background: 'transparent',
                    border: '1px solid var(--color-muted)',
                    color: 'var(--color-text)',
                    fontFamily: 'var(--font-display)',
                    fontSize: '12px',
                    fontWeight: 600,
                    letterSpacing: '1px',
                    cursor: 'pointer'
                  }}
                >
                  BACK
                </button>
              )}
              
              {currentStep !== "validation" && (
                <button
                  onClick={handleNext}
                  disabled={
                    (currentStep === "org" && !state.orgName) ||
                    (currentStep === "ingestion" && !state.ingestionMethod)
                  }
                  className="px-6 py-3"
                  style={{
                    background: 'var(--color-primary)',
                    border: '1px solid var(--color-primary)',
                    color: '#050506',
                    fontFamily: 'var(--font-display)',
                    fontSize: '12px',
                    fontWeight: 600,
                    letterSpacing: '1px',
                    cursor: 'pointer',
                    opacity: (currentStep === "org" && !state.orgName) || (currentStep === "ingestion" && !state.ingestionMethod) ? 0.5 : 1
                  }}
                >
                  NEXT
                </button>
              )}
              
              {currentStep === "validation" && state.validationStatus === "success" && (
                <button
                  onClick={() => window.location.href = '/dashboard'}
                  className="px-6 py-3"
                  style={{
                    background: 'var(--color-primary)',
                    border: '1px solid var(--color-primary)',
                    color: '#050506',
                    fontFamily: 'var(--font-display)',
                    fontSize: '12px',
                    fontWeight: 600,
                    letterSpacing: '1px',
                    cursor: 'pointer'
                  }}
                >
                  ENTER DASHBOARD
                </button>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
