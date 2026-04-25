// Package tools – builtins.go
// Ships default tools that are available out of the box.
package tools

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// RegisterBuiltins adds the standard tool set to the registry.
func RegisterBuiltins(r *Registry) {
	r.Register(calculatorTool())
	r.Register(dateTimeTool())
	r.Register(textStatsTool())
}

// ── Calculator ──────────────────────────────────────────────────────────────

func calculatorTool() Tool {
	return Tool{
		Schema: ToolSchema{
			Name:        "calculator",
			Description: "Evaluates simple arithmetic expressions (+, -, *, /, ^, sqrt).",
			Params: []ParamSchema{
				{Name: "expression", Type: "string", Description: "The math expression to evaluate", Required: true},
			},
			ReturnType: "string",
		},
		Fn: func(_ context.Context, inputs map[string]string) (string, error) {
			expr := inputs["expression"]
			// Simple two-operand parser for demo; real impl would use a proper parser.
			for _, op := range []string{"**", "^", "*", "/", "+", "-"} {
				parts := strings.SplitN(expr, op, 2)
				if len(parts) == 2 {
					a, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
					b, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
					if err1 != nil || err2 != nil {
						continue
					}
					var result float64
					switch op {
					case "+":
						result = a + b
					case "-":
						result = a - b
					case "*":
						result = a * b
					case "/":
						if b == 0 {
							return "error: division by zero", nil
						}
						result = a / b
					case "^", "**":
						result = math.Pow(a, b)
					}
					return fmt.Sprintf("%.6g", result), nil
				}
			}
			// Try sqrt
			if strings.HasPrefix(expr, "sqrt(") && strings.HasSuffix(expr, ")") {
				inner := expr[5 : len(expr)-1]
				n, err := strconv.ParseFloat(strings.TrimSpace(inner), 64)
				if err == nil {
					return fmt.Sprintf("%.6g", math.Sqrt(n)), nil
				}
			}
			return "error: could not parse expression", nil
		},
	}
}

// ── DateTime ────────────────────────────────────────────────────────────────

func dateTimeTool() Tool {
	return Tool{
		Schema: ToolSchema{
			Name:        "datetime",
			Description: "Returns the current date/time in the specified timezone.",
			Params: []ParamSchema{
				{Name: "timezone", Type: "string", Description: "IANA timezone (e.g. Asia/Kolkata, UTC)", Required: false},
			},
			ReturnType: "string",
		},
		Fn: func(_ context.Context, inputs map[string]string) (string, error) {
			tz := inputs["timezone"]
			if tz == "" {
				tz = "UTC"
			}
			loc, err := time.LoadLocation(tz)
			if err != nil {
				return "", fmt.Errorf("invalid timezone %q: %w", tz, err)
			}
			now := time.Now().In(loc)
			return now.Format("2006-01-02 15:04:05 MST"), nil
		},
	}
}

// ── Text Stats ──────────────────────────────────────────────────────────────

func textStatsTool() Tool {
	return Tool{
		Schema: ToolSchema{
			Name:        "text_stats",
			Description: "Counts words, characters, and sentences in the given text.",
			Params: []ParamSchema{
				{Name: "text", Type: "string", Description: "The text to analyze", Required: true},
			},
			ReturnType: "json",
		},
		Fn: func(_ context.Context, inputs map[string]string) (string, error) {
			text := inputs["text"]
			words := len(strings.Fields(text))
			chars := len(text)
			sentences := strings.Count(text, ".") + strings.Count(text, "!") + strings.Count(text, "?")
			return fmt.Sprintf(`{"words":%d,"characters":%d,"sentences":%d}`, words, chars, sentences), nil
		},
	}
}
