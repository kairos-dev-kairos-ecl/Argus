import { createBrowserRouter } from "react-router";
import { Layout } from "./components/Layout";
import { OnboardingFlow } from "./components/OnboardingFlow";
import { AuditLedger } from "./components/AuditLedger";
import { EnhancedTraceFlow } from "./components/EnhancedTraceFlow";
import { EnhancedTelemetryDashboard } from "./components/EnhancedTelemetryDashboard";
import { EnhancedHuntingConsole } from "./components/EnhancedHuntingConsole";

export const router = createBrowserRouter([
  {
    path: "/",
    Component: Layout,
    children: [
      { index: true, Component: OnboardingFlow },
      { path: "trace", Component: EnhancedTraceFlow },
      { path: "telemetry", Component: EnhancedTelemetryDashboard },
      { path: "hunt", Component: EnhancedHuntingConsole },
      { path: "audit", Component: AuditLedger },
    ],
  },
]);
