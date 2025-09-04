package workflow

// Simple expression parser that handles the basic expression evaluation
// needed by the dependency resolver. This replaces the over-engineered
// AST-based parser that was unused.

import (
	"joblet/internal/joblet/domain"
	"strings"
)

// SimpleExpressionEvaluator provides basic expression evaluation for job dependencies
type SimpleExpressionEvaluator struct {
	jobStateCache map[string]domain.JobStatus
}

// NewSimpleExpressionEvaluator creates a new simple expression evaluator
func NewSimpleExpressionEvaluator(jobStates map[string]domain.JobStatus) *SimpleExpressionEvaluator {
	return &SimpleExpressionEvaluator{
		jobStateCache: jobStates,
	}
}

// Evaluate evaluates a dependency expression using simple string parsing
// Supports: "job=status", "job1=status AND job2=status", "job1=status OR job2=status"
// Also supports: "job IN (status1,status2,status3)"
func (e *SimpleExpressionEvaluator) Evaluate(expr string) bool {
	return e.parseAndEvaluate(strings.TrimSpace(expr))
}

// parseAndEvaluate recursively parses and evaluates expressions
func (e *SimpleExpressionEvaluator) parseAndEvaluate(expr string) bool {
	expr = strings.TrimSpace(expr)

	// Handle parentheses recursively
	if strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
		return e.parseAndEvaluate(expr[1 : len(expr)-1])
	}

	// Handle OR expressions (lowest precedence)
	if strings.Contains(expr, " OR ") {
		parts := strings.Split(expr, " OR ")
		for _, part := range parts {
			if e.parseAndEvaluate(strings.TrimSpace(part)) {
				return true
			}
		}
		return false
	}

	// Handle AND expressions (higher precedence)
	if strings.Contains(expr, " AND ") {
		parts := strings.Split(expr, " AND ")
		for _, part := range parts {
			if !e.parseAndEvaluate(strings.TrimSpace(part)) {
				return false
			}
		}
		return true
	}

	// Handle IN expressions: "job IN (status1,status2)"
	if strings.Contains(expr, " IN ") {
		return e.evaluateInExpression(expr)
	}

	// Handle simple equality: "job=status"
	if strings.Contains(expr, "=") {
		return e.evaluateSimpleComparison(expr)
	}

	return false
}

// evaluateInExpression handles "job IN (status1,status2,status3)" expressions
func (e *SimpleExpressionEvaluator) evaluateInExpression(expr string) bool {
	parts := strings.Split(expr, " IN ")
	if len(parts) != 2 {
		return false
	}

	jobName := strings.TrimSpace(parts[0])
	statusList := strings.TrimSpace(parts[1])

	// Remove parentheses and split by comma
	statusList = strings.Trim(statusList, "()")
	statuses := strings.Split(statusList, ",")

	currentStatus, exists := e.jobStateCache[jobName]
	if !exists {
		return false
	}

	for _, status := range statuses {
		if strings.TrimSpace(status) == string(currentStatus) {
			return true
		}
	}
	return false
}

// evaluateSimpleComparison handles "job=status" expressions
func (e *SimpleExpressionEvaluator) evaluateSimpleComparison(expr string) bool {
	parts := strings.Split(expr, "=")
	if len(parts) != 2 {
		return false
	}

	jobName := strings.TrimSpace(parts[0])
	expectedStatus := strings.TrimSpace(parts[1])

	currentStatus, exists := e.jobStateCache[jobName]
	if !exists {
		return false
	}

	return string(currentStatus) == expectedStatus
}
