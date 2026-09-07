"use client";

import { Component, type ReactNode } from "react";

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
      <div
        className="min-h-screen flex items-center justify-center p-6"
        style={{ background: "var(--background)" }}
      >
        <div
          className="w-full max-w-md rounded-2xl p-8 space-y-5 text-center"
          style={{
            background: "var(--surface)",
            border: "1px solid var(--border)",
          }}
        >
          <div
            className="mx-auto flex items-center justify-center w-12 h-12 rounded-full"
            style={{ background: "#ef444418" }}
          >
            <span className="text-xl" style={{ color: "var(--error)" }}>
              !
            </span>
          </div>

          <h2
            className="text-lg font-semibold"
            style={{ color: "var(--foreground)" }}
          >
            Something went wrong
          </h2>

          <p className="text-sm" style={{ color: "var(--muted)" }}>
            An unexpected error occurred. Please refresh the page and try again.
          </p>

          <p
            className="text-xs font-mono px-3 py-2 rounded-lg"
            style={{
              background: "#ef444410",
              color: "var(--error)",
              border: "1px solid #ef444425",
              wordBreak: "break-word",
            }}
          >
            {this.state.message}
          </p>

          <button
            onClick={() => {
              this.setState({ hasError: false, message: "" });
              window.location.reload();
            }}
            className="px-5 py-2 rounded-lg text-sm font-medium transition-opacity"
            style={{ background: "var(--accent)", color: "#fff" }}
          >
            Reload page
          </button>
        </div>
      </div>
    );
  }
}
