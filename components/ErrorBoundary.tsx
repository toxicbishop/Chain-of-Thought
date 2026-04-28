"use client";

import { Component, type ReactNode } from "react";
import { AlertTriangle, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  message: string;
}

export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, message: "" };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, message: error.message || "Something went wrong" };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error("[ErrorBoundary]", error, info.componentStack);
  }

  render() {
    if (!this.state.hasError) return this.props.children;

    return (
      <div className="min-h-screen flex items-center justify-center p-6 bg-background">
        <div className="w-full max-w-md rounded-2xl p-8 space-y-5 text-center bg-surface border border-border shadow-elegant">
          <div className="mx-auto flex items-center justify-center w-12 h-12 rounded-full bg-destructive/10 text-destructive">
            <AlertTriangle size={22} />
          </div>
          <h2 className="text-lg font-semibold text-foreground">Something went wrong</h2>
          <p className="text-sm text-muted-foreground">
            An unexpected error occurred. Please refresh the page and try again.
          </p>
          <p className="text-xs font-mono px-3 py-2 rounded-lg bg-destructive/10 text-destructive border border-destructive/25 break-words">
            {this.state.message}
          </p>
          <Button
            onClick={() => {
              this.setState({ hasError: false, message: "" });
              window.location.reload();
            }}
            className="w-full"
          >
            <RefreshCw size={16} className="mr-2" />
            Reload page
          </Button>
        </div>
      </div>
    );
  }
}
