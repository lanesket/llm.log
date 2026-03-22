import { lazy, Suspense } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { Loader2Icon } from "lucide-react";
import { TimeRangeProvider } from "@/hooks/useTimeRange";
import { ChameleonProvider } from "@/hooks/useChameleon";
import { Layout } from "@/components/Layout";
import { Chameleon } from "@/components/Chameleon";
import { Requests } from "@/pages/Requests";

// Lazy-load pages that import recharts (~200KB)
const Dashboard = lazy(() => import("@/pages/Dashboard"));
const Analytics = lazy(() => import("@/pages/Analytics"));
const RequestDetail = lazy(() => import("@/pages/RequestDetail"));

function PageLoader() {
  return (
    <div className="flex h-64 items-center justify-center">
      <Loader2Icon className="size-6 animate-spin text-muted-foreground" />
    </div>
  );
}

function AppInner() {
  return (
    <ChameleonProvider>
      <Chameleon />
      <Suspense fallback={<PageLoader />}>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/requests" element={<Requests />} />
            <Route path="/requests/:id" element={<RequestDetail />} />
            <Route path="/analytics" element={<Analytics />} />
          </Route>
        </Routes>
      </Suspense>
    </ChameleonProvider>
  );
}

export default function App() {
  return (
    <TimeRangeProvider>
      <BrowserRouter>
        <AppInner />
      </BrowserRouter>
    </TimeRangeProvider>
  );
}
