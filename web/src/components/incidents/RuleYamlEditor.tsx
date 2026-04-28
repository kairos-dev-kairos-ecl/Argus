import { useRef, useEffect, useState } from 'react';
import { EditorView } from '@codemirror/view';
import { EditorState } from '@codemirror/state';
import { yaml } from '@codemirror/lang-yaml';
import { oneDark } from '@codemirror/theme-one-dark';

const DEFAULT_YAML = `name: detect_prompt_injection
severity: 3
conditions:
  - layer: 8
  - category: guardrail.injection
`;

interface RuleYamlEditorProps {
  initialYaml?: string;
  onSubmit: (yaml: string, name: string) => Promise<void>;
}

export function RuleYamlEditor({ initialYaml, onSubmit }: RuleYamlEditorProps) {
  const editorRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const [yamlContent, setYamlContent] = useState(initialYaml ?? DEFAULT_YAML);
  const [ruleName, setRuleName] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (!editorRef.current) return;

    const state = EditorState.create({
      doc: initialYaml ?? DEFAULT_YAML,
      extensions: [
        yaml(),
        oneDark,
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            setYamlContent(update.state.doc.toString());
          }
        }),
        EditorView.theme({
          '&': {
            background: 'var(--color-background)',
            fontFamily: 'var(--font-mono)',
            fontSize: '12px',
          },
          '.cm-content': {
            padding: '8px 0',
          },
          '.cm-scroller': {
            overflow: 'auto',
          },
        }),
      ],
    });

    const view = new EditorView({
      state,
      parent: editorRef.current,
    });

    viewRef.current = view;

    return () => {
      view.destroy();
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleSubmit = async () => {
    if (!ruleName.trim() || !yamlContent.trim()) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      await onSubmit(yamlContent, ruleName.trim());
      setRuleName('');
    } catch (e) {
      setSubmitError((e as Error)?.message ?? 'Submit failed');
    } finally {
      setSubmitting(false);
    }
  };

  const canSubmit = ruleName.trim().length > 0 && yamlContent.trim().length > 0 && !submitting;

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        flex: 1,
        minHeight: 0,
        borderTop: 'var(--border-stark)',
      }}
    >
      {/* Header */}
      <div
        style={{
          padding: '8px 12px',
          borderBottom: 'var(--border-stark)',
          fontFamily: 'var(--font-mono)',
          fontSize: '11px',
          color: 'var(--color-muted)',
          textTransform: 'uppercase',
          letterSpacing: '0.05em',
          background: 'var(--color-surface)',
          flexShrink: 0,
        }}
      >
        RULE YAML
      </div>

      {/* CodeMirror editor */}
      <div
        ref={editorRef}
        style={{
          flex: 1,
          overflow: 'auto',
          background: 'var(--color-background)',
          minHeight: '120px',
        }}
      />

      {/* Footer: name input + submit */}
      <div
        style={{
          display: 'flex',
          gap: '8px',
          padding: '8px 12px',
          borderTop: 'var(--border-stark)',
          background: 'var(--color-surface)',
          flexShrink: 0,
          alignItems: 'center',
        }}
      >
        <input
          type="text"
          placeholder="RULE NAME"
          value={ruleName}
          onChange={(e) => setRuleName(e.target.value)}
          style={{
            flex: 1,
            background: 'var(--color-background)',
            border: 'var(--border-stark)',
            fontFamily: 'var(--font-mono)',
            fontSize: '11px',
            padding: '4px 8px',
            color: 'var(--color-text)',
            outline: 'none',
          }}
        />
        <button
          onClick={handleSubmit}
          disabled={!canSubmit}
          style={{
            padding: '4px 16px',
            fontFamily: 'var(--font-mono)',
            fontSize: '11px',
            textTransform: 'uppercase',
            letterSpacing: '0.05em',
            background: 'transparent',
            border: canSubmit ? '1px solid var(--color-primary)' : 'var(--border-stark)',
            color: canSubmit ? 'var(--color-primary)' : 'var(--color-muted)',
            cursor: canSubmit ? 'pointer' : 'not-allowed',
            transition: 'all 100ms',
          }}
        >
          {submitting ? 'SUBMITTING...' : 'SUBMIT'}
        </button>
      </div>

      {submitError && (
        <div
          style={{
            padding: '4px 12px',
            fontFamily: 'var(--font-mono)',
            fontSize: '10px',
            color: 'var(--color-alert)',
            borderTop: 'var(--border-stark)',
            background: 'var(--color-surface)',
          }}
        >
          ERROR: {submitError}
        </div>
      )}
    </div>
  );
}
