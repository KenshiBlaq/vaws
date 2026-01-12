package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"vaws/internal/model"
)

// ListFunctions lists all Lambda functions.
func (c *Client) ListFunctions(ctx context.Context) ([]model.Function, error) {
	var functions []model.Function

	paginator := lambda.NewListFunctionsPaginator(c.lambda, &lambda.ListFunctionsInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list Lambda functions: %w", err)
		}

		for _, fn := range page.Functions {
			functions = append(functions, convertFunction(fn))
		}
	}

	return functions, nil
}

// ListFunctionsPagedCallback lists Lambda functions with a callback for each page.
// This enables lazy loading by delivering results incrementally.
// The callback receives the functions from each page and returns true to continue or false to stop.
func (c *Client) ListFunctionsPagedCallback(ctx context.Context, callback func(functions []model.Function, hasMore bool) bool) error {
	paginator := lambda.NewListFunctionsPaginator(c.lambda, &lambda.ListFunctionsInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list Lambda functions: %w", err)
		}

		var functions []model.Function
		for _, fn := range page.Functions {
			functions = append(functions, convertFunction(fn))
		}

		hasMore := paginator.HasMorePages()
		if !callback(functions, hasMore) {
			break
		}
	}

	return nil
}

// DescribeFunction returns detailed information about a Lambda function.
func (c *Client) DescribeFunction(ctx context.Context, functionName string) (*model.Function, error) {
	out, err := c.lambda.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: aws.String(functionName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe function %s: %w", functionName, err)
	}

	fn := convertFunctionConfig(*out.Configuration)
	return &fn, nil
}

// InvokeFunction invokes a Lambda function with the given payload.
// Returns the invocation result including response payload and execution metadata.
func (c *Client) InvokeFunction(ctx context.Context, functionName, payload string) (*model.InvocationResult, error) {
	start := time.Now()

	input := &lambda.InvokeInput{
		FunctionName: aws.String(functionName),
		LogType:      types.LogTypeTail, // Get last 4KB of execution log
	}

	// Only set payload if non-empty
	if payload != "" {
		input.Payload = []byte(payload)
	}

	out, err := c.lambda.Invoke(ctx, input)
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("failed to invoke function %s: %w", functionName, err)
	}

	result := &model.InvocationResult{
		FunctionName:    functionName,
		StatusCode:      int(out.StatusCode),
		ExecutedVersion: aws.ToString(out.ExecutedVersion),
		Payload:         string(out.Payload),
		Duration:        duration,
		InvokedAt:       start,
	}

	if out.FunctionError != nil {
		result.FunctionError = *out.FunctionError
	}

	if out.LogResult != nil {
		result.LogResult = *out.LogResult
	}

	return result, nil
}

// convertFunction converts an AWS Lambda function configuration to our model.
func convertFunction(fn types.FunctionConfiguration) model.Function {
	return convertFunctionConfig(fn)
}

// convertFunctionConfig converts AWS Lambda FunctionConfiguration to our model.
func convertFunctionConfig(fn types.FunctionConfiguration) model.Function {
	function := model.Function{
		Name:        aws.ToString(fn.FunctionName),
		ARN:         aws.ToString(fn.FunctionArn),
		Runtime:     string(fn.Runtime),
		Handler:     aws.ToString(fn.Handler),
		MemorySize:  int(aws.ToInt32(fn.MemorySize)),
		Timeout:     int(aws.ToInt32(fn.Timeout)),
		CodeSize:    fn.CodeSize,
		Description: aws.ToString(fn.Description),
		Role:        aws.ToString(fn.Role),
		PackageType: string(fn.PackageType),
	}

	// Parse LastModified timestamp
	if fn.LastModified != nil {
		if t, err := time.Parse("2006-01-02T15:04:05.000+0000", *fn.LastModified); err == nil {
			function.LastModified = t
		}
	}

	// Map function state
	if fn.State != "" {
		switch fn.State {
		case "Active":
			function.State = model.FunctionStateActive
		case "Pending":
			function.State = model.FunctionStatePending
		case "Inactive":
			function.State = model.FunctionStateInactive
		case "Failed":
			function.State = model.FunctionStateFailed
		default:
			function.State = model.FunctionStateActive // Default to active if state is empty
		}
	} else {
		function.State = model.FunctionStateActive // Lambda functions are active by default
	}

	return function
}
