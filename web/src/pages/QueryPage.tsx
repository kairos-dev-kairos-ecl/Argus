import React from 'react'
import { QueryInterface } from '../components/QueryInterface'

/**
 * Page component for SQL query interface.
 * Routes: /query
 * Provides a free-form SQL editor for ClickHouse queries against signals table.
 */
export const QueryPage: React.FC = () => {
  return (
    <div className="flex flex-col h-full">
      <div className="px-4 pt-4 pb-2">
        <h1 className="text-2xl font-bold text-foreground">Signal Query</h1>
        <p className="text-muted-foreground text-sm mt-1">
          Execute SQL queries against the signals table with autocomplete and pagination
        </p>
      </div>
      <QueryInterface />
    </div>
  )
}
