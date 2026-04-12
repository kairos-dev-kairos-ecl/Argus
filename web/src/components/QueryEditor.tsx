import React, { useState, useEffect, useRef } from 'react'
import { EditorView, lineNumbers } from '@codemirror/view'
import { EditorState } from '@codemirror/state'
import { sql as sqlLanguage } from '@codemirror/lang-sql'
import { oneDark } from '@codemirror/theme-one-dark'
import { autocompletion, type CompletionContext } from '@codemirror/autocomplete'
import { keymap } from '@codemirror/view'
import axios from 'axios'

interface QueryEditorProps {
  sql: string
  onChange: (sql: string) => void
  onExecute: () => void
}

/**
 * SQL editor component powered by CodeMirror v6.
 * Features:
 * - SQL syntax highlighting
 * - Schema-aware autocomplete from ClickHouse
 * - Ctrl+Enter to execute
 */
export const QueryEditor: React.FC<QueryEditorProps> = ({ sql, onChange, onExecute }) => {
  const editorRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<EditorView | null>(null)
  const [schema, setSchema] = useState<Record<string, string>>({})
  const [schemaLoading, setSchemaLoading] = useState(false)

  // Load ClickHouse schema on mount
  useEffect(() => {
    const loadSchema = async () => {
      try {
        setSchemaLoading(true)
        const response = await axios.get('/api/v1/schema/signals')
        setSchema(response.data || {})
      } catch (error) {
        console.error('Failed to load schema:', error)
        setSchema({})
      } finally {
        setSchemaLoading(false)
      }
    }

    loadSchema()
  }, [])

  // Initialize CodeMirror editor
  useEffect(() => {
    if (!editorRef.current) return

    const sqlAutocomplete = (context: CompletionContext) => {
      const word = context.matchBefore(/\w*/)
      if (!word || (word.from === word.to && !context.explicit)) {
        return null
      }

      const completions = Object.entries(schema).map(([name, type]) => ({
        label: name,
        type: 'variable' as const,
        info: type,
        boost: 1
      }))

      return {
        from: word.from,
        options: completions
      }
    }

    const executeKeymap = keymap.of([
      {
        key: 'Ctrl-Enter',
        mac: 'Cmd-Enter',
        run: () => {
          onExecute()
          return true
        }
      }
    ])

    const state = EditorState.create({
      doc: sql,
      extensions: [
        lineNumbers(),
        sqlLanguage(),
        oneDark,
        autocompletion({
          override: [sqlAutocomplete]
        }),
        executeKeymap,
        EditorView.updateListener.of(update => {
          if (update.docChanged) {
            onChange(update.state.doc.toString())
          }
        })
      ]
    })

    const editorView = new EditorView({
      state,
      parent: editorRef.current
    })

    viewRef.current = editorView

    return () => {
      editorView.destroy()
    }
  }, [schema, onChange, onExecute])

  // Update editor when sql prop changes externally
  useEffect(() => {
    if (viewRef.current && sql !== viewRef.current.state.doc.toString()) {
      viewRef.current.dispatch({
        changes: {
          from: 0,
          to: viewRef.current.state.doc.length,
          insert: sql
        }
      })
    }
  }, [sql])

  return (
    <div className="flex flex-col h-full bg-muted-background rounded border border-border overflow-hidden">
      {/* Header */}
      <div className="p-3 flex items-center justify-between bg-background border-b border-border">
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">SQL Editor</span>
          {schemaLoading && <span className="text-xs text-muted-foreground animate-pulse">Loading schema...</span>}
        </div>
        <button
          onClick={onExecute}
          className="h-8 px-3 bg-primary hover:bg-primary/90 text-foreground text-sm font-medium rounded transition-colors duration-200"
          aria-label="Execute query (Ctrl+Enter)"
        >
          Execute (Ctrl+Enter)
        </button>
      </div>

      {/* Editor container */}
      <div className="flex-1 overflow-hidden" ref={editorRef} />
    </div>
  )
}
