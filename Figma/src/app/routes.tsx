import { createBrowserRouter } from "react-router";
import { Layout } from "./components/Layout";
import { OnboardingFlow } from "./components/OnboardingFlow";
import { TraceWaterfall } from "./components/TraceWaterfall";
import { SystemHealthDashboard } from "./components/SystemHealthDashboard";
import { HuntingConsole } from "./components/HuntingConsole";
import { AuditLedger } from "./components/AuditLedger";

export const router = createBrowserRouter([
  {
    path: "/",
    Component: Layout,
    children: [
      { index: true, Component: OnboardingFlow },
      { path: "trace", Component: TraceWaterfall },
      { path: "dashboard", Component: SystemHealthDashboard },
      { path: "hunt", Component: HuntingConsole },
      { path: "audit", Component: AuditLedger },
    ],
  },
]);
