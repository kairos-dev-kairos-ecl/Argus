# Argus XDR User Guide

## Introduction

Argus XDR is an Extended Detection & Response platform purpose-built for LLM-integrated systems. This guide covers all features and workflows for operators.

## Getting Started

### Dashboard

The Dashboard is your primary view into system health and real-time signal activity.

**Coverage Map**: Displays the health status of all 10 layers (L1-L10):
- **Green**: Layer is healthy, receiving signals regularly
- **Yellow**: Layer has reduced signal rate
- **Red**: Layer has errors or no recent signals
- **Gray**: Layer has never received signals

**Signal Stream**: Live feed of incoming signals, filterable by layer and severity.

### Navigation

Use the main navigation menu (sidebar on desktop, hamburger menu on mobile) to access:
- Dashboard: Main signal view
- Query: SQL editor for ad-hoc analysis
- Configuration: System settings and rule editor
- Users: User management (admin only)
- Audit Log: Immutable audit trail (admin/analyst only)

## Workflows

### Investigating a Signal

1. **From Dashboard**: Click a signal in the signal stream
2. **View Details**: Signal details panel shows:
   - Timestamp and layer
   - Event data and context
   - Associated detections (if any)
   - Trace ID (click to view full trace)

3. **View Trace**: Click trace ID to see end-to-end flow
   - Timeline shows span relationships
   - Click spans to see detailed context
   - View detected anomalies in context

### Running Queries

1. **Navigate to Query page**
2. **Write SQL**: Edit SQL in the CodeMirror editor
   - Autocomplete available for schema fields
   - Press Ctrl+Enter (or click Execute) to run
3. **Review Results**: Results table with pagination
4. **Export**: Click "Export CSV" to download data

### Managing Rules

*Available to admin users only*

1. **Navigate to Configuration > Rules**
2. **Edit YAML**: Write or paste rules in the editor
3. **Validate**: Click "Validate" to check syntax
4. **Apply**: Click "Apply" to activate rules
5. **View Detections**: Navigate to dashboard to see detections

### User Management

*Available to admin users only*

1. **Navigate to Users**
2. **Create User**: Click "Create User" button
   - Enter email, display name, and role
   - Roles: Admin (full access), Analyst (read+investigate), Viewer (read-only)
3. **Manage Roles**: Use role dropdown to change user role
4. **Suspend User**: Click "Suspend" to disable account
5. **Reset Password**: Click "Reset PW" to send reset email

### Audit Log

*Available to admin and analyst users*

1. **Navigate to Audit Log**
2. **Filter**: Use filter controls to narrow results
   - By action (login, create_rule, etc.)
   - By user
   - By date range
3. **Export**: Click "Export CSV" for compliance reporting

## Accessibility

Argus XDR is designed for accessibility:

- **Keyboard Navigation**: Tab through interactive elements
- **Focus Indicators**: 2px blue outline shows focused element
- **ARIA Labels**: Screen readers announce button purposes
- **Touch Targets**: All buttons and interactive elements are at least 44px

### Keyboard Shortcuts

- Tab: Navigate to next element
- Shift+Tab: Navigate to previous element
- Enter: Activate buttons/links
- Escape: Close modals/dropdowns

## Responsive Design

Argus XDR works seamlessly on:
- Mobile (375px): Single-column layout, stacked controls
- Tablet (768px): Two-column layout where applicable
- Desktop (1280px+): Full multi-column layout

## Support

For issues, contact your system administrator.
