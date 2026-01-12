package ui

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	tea "github.com/charmbracelet/bubbletea"

	"vaws/internal/model"
)

// fetchCloudWatchLogs fetches CloudWatch logs for the selected container.
func (m *Model) fetchCloudWatchLogs() tea.Cmd {
	config := m.cloudWatchLogsPanel.SelectedContainer()
	if config == nil {
		return nil
	}

	startTime := m.state.CloudWatchLastFetchTime

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		entries, lastTimestamp, err := m.client.FetchLogs(
			ctx,
			config.LogGroup,
			config.LogStreamName,
			startTime,
			100, // Limit per fetch
		)

		return cloudWatchLogsLoadedMsg{
			entries:       entries,
			lastTimestamp: lastTimestamp,
			err:           err,
		}
	}
}

// fetchLambdaCloudWatchLogs fetches CloudWatch logs for a Lambda function.
func (m *Model) fetchLambdaCloudWatchLogs(logGroup string) tea.Cmd {
	startTime := m.state.CloudWatchLastFetchTime

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		entries, lastTimestamp, err := m.client.FetchLambdaLogs(
			ctx,
			logGroup,
			startTime,
			100, // Limit per fetch
		)

		return cloudWatchLogsLoadedMsg{
			entries:       entries,
			lastTimestamp: lastTimestamp,
			err:           err,
		}
	}
}

// loadStacks loads CloudFormation stacks.
func (m *Model) loadStacks() tea.Cmd {
	m.state.StacksLoading = true
	m.stacksList.SetLoading(true)
	m.splash.SetLoading("Loading CloudFormation stacks...")
	m.logger.Info("Loading CloudFormation stacks...")

	return tea.Batch(
		m.splash.Spinner().TickCmd(), // Ensure spinner keeps ticking
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			stacks, err := m.client.ListStacks(ctx)
			return stacksLoadedMsg{stacks: stacks, err: err}
		},
	)
}

// loadServices loads ECS services for the selected stack.
func (m *Model) loadServices() tea.Cmd {
	if m.state.SelectedStack == nil {
		return nil
	}

	m.state.ServicesLoading = true
	m.serviceList.SetLoading(true)
	stackName := m.state.SelectedStack.Name
	m.logger.Info("Loading ECS services for stack: %s", stackName)

	return tea.Batch(
		m.serviceList.Spinner().TickCmd(), // Ensure spinner keeps ticking
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			services, err := m.client.GetServicesForStack(ctx, stackName)
			return servicesLoadedMsg{services: services, err: err}
		},
	)
}

// loadServicesForCluster loads ECS services for the selected cluster.
func (m *Model) loadServicesForCluster() tea.Cmd {
	if m.state.SelectedCluster == nil {
		return nil
	}

	m.state.ServicesLoading = true
	m.serviceList.SetLoading(true)
	clusterARN := m.state.SelectedCluster.ARN
	clusterName := m.state.SelectedCluster.Name
	m.logger.Info("Loading ECS services for cluster: %s", clusterName)

	return tea.Batch(
		m.serviceList.Spinner().TickCmd(),
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			services, err := m.client.ListServices(ctx, clusterARN)
			return servicesLoadedMsg{services: services, err: err}
		},
	)
}

// loadFunctions loads Lambda functions with lazy loading.
func (m *Model) loadFunctions() tea.Cmd {
	m.state.FunctionsLoading = true
	m.lambdaList.SetLoading(true)

	// Check if a stack is selected - if so, only load functions from that stack
	var stackName string
	if m.state.SelectedStack != nil {
		stackName = m.state.SelectedStack.Name
		m.logger.Info("Loading Lambda functions for stack: %s", stackName)
	} else {
		m.logger.Info("Loading all Lambda functions...")
	}

	// Use channel to receive incremental results
	resultChan := make(chan functionsLoadedMsg, 10)

	// Start background loading
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		defer close(resultChan)

		if stackName != "" {
			// Stack-specific loading (no pagination needed, usually small)
			functionNames, err := m.client.GetLambdaFunctionsFromStack(ctx, stackName)
			if err != nil {
				resultChan <- functionsLoadedMsg{functions: nil, err: err}
				return
			}

			if len(functionNames) == 0 {
				resultChan <- functionsLoadedMsg{functions: []model.Function{}, err: nil}
				return
			}

			var functions []model.Function
			for _, name := range functionNames {
				fn, err := m.client.DescribeFunction(ctx, name)
				if err != nil {
					continue
				}
				functions = append(functions, *fn)
			}
			resultChan <- functionsLoadedMsg{functions: functions, err: nil}
			return
		}

		// Lazy load with incremental results
		isFirst := true
		err := m.client.ListFunctionsPagedCallback(ctx, func(functions []model.Function, hasMore bool) bool {
			resultChan <- functionsLoadedMsg{
				functions: functions,
				err:       nil,
				hasMore:   hasMore,
				isAppend:  !isFirst,
			}
			isFirst = false
			return true // continue loading
		})
		if err != nil {
			resultChan <- functionsLoadedMsg{functions: nil, err: err}
		}
	}()

	// Return command that reads from channel
	return tea.Batch(
		m.lambdaList.Spinner().TickCmd(),
		func() tea.Msg {
			msg, ok := <-resultChan
			if !ok {
				return nil
			}
			// Store channel for subsequent reads
			m.functionsResultChan = resultChan
			return msg
		},
	)
}

// continueFunctionsLoad continues reading from the functions result channel.
func (m *Model) continueFunctionsLoad() tea.Cmd {
	if m.functionsResultChan == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-m.functionsResultChan
		if !ok {
			m.functionsResultChan = nil
			return nil
		}
		return msg
	}
}

// loadAPIs loads API Gateway REST and HTTP APIs.
func (m *Model) loadAPIs() tea.Cmd {
	m.state.APIsLoading = true
	m.apiGatewayList.SetLoading(true)

	// Check if a stack is selected - if so, only load APIs from that stack
	var stackName string
	if m.state.SelectedStack != nil {
		stackName = m.state.SelectedStack.Name
		m.logger.Info("Loading API Gateway APIs for stack: %s", stackName)
	} else {
		m.logger.Info("Loading all API Gateway APIs...")
	}

	return tea.Batch(
		m.apiGatewayList.Spinner().TickCmd(),
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if stackName != "" {
				// Get API IDs from the stack
				restAPIIDs, _, err := m.client.GetAPIGatewaysFromStack(ctx, stackName)
				if err != nil {
					return restAPIsLoadedMsg{apis: nil, err: err}
				}

				// If no REST APIs in stack, return empty list
				if len(restAPIIDs) == 0 {
					return restAPIsLoadedMsg{apis: []model.RestAPI{}, err: nil}
				}

				// Get details for each API
				var apis []model.RestAPI
				for _, id := range restAPIIDs {
					api, err := m.client.GetRestAPI(ctx, id)
					if err != nil {
						continue
					}
					apis = append(apis, *api)
				}
				return restAPIsLoadedMsg{apis: apis, err: nil}
			}

			restAPIs, err := m.client.ListRestAPIs(ctx)
			return restAPIsLoadedMsg{apis: restAPIs, err: err}
		},
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if stackName != "" {
				// Get API IDs from the stack
				_, httpAPIIDs, err := m.client.GetAPIGatewaysFromStack(ctx, stackName)
				if err != nil {
					return httpAPIsLoadedMsg{apis: nil, err: err}
				}

				// If no HTTP APIs in stack, return empty list
				if len(httpAPIIDs) == 0 {
					return httpAPIsLoadedMsg{apis: []model.HttpAPI{}, err: nil}
				}

				// Get details for each API
				var apis []model.HttpAPI
				for _, id := range httpAPIIDs {
					api, err := m.client.GetHttpAPI(ctx, id)
					if err != nil {
						continue
					}
					apis = append(apis, *api)
				}
				return httpAPIsLoadedMsg{apis: apis, err: nil}
			}

			httpAPIs, err := m.client.ListHttpAPIs(ctx)
			return httpAPIsLoadedMsg{apis: httpAPIs, err: err}
		},
	)
}

// loadEC2Instances loads SSM-managed EC2 instances for jump host selection.
func (m *Model) loadEC2Instances() tea.Cmd {
	m.state.EC2InstancesLoading = true
	m.ec2List.SetLoading(true)
	m.logger.Info("Loading SSM-managed EC2 instances...")

	return tea.Batch(
		m.ec2List.Spinner().TickCmd(),
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			instances, err := m.client.ListSSMManagedInstances(ctx)
			return ec2InstancesLoadedMsg{instances: instances, err: err}
		},
	)
}

// loadQueues loads SQS queues with lazy loading.
func (m *Model) loadQueues() tea.Cmd {
	m.state.QueuesLoading = true
	m.sqsTable.SetLoading(true)

	// Check if a stack is selected - if so, only load queues from that stack
	var stackName string
	if m.state.SelectedStack != nil {
		stackName = m.state.SelectedStack.Name
		m.logger.Info("Loading SQS queues for stack: %s", stackName)
	} else {
		m.logger.Info("Loading all SQS queues...")
	}

	// Use channel for incremental results
	resultChan := make(chan queuesLoadedMsg, 10)

	// Start background loading
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		defer close(resultChan)

		if stackName != "" {
			// Stack-specific loading (no pagination, usually small)
			queueURLs, err := m.client.GetQueuesFromStack(ctx, stackName)
			if err != nil {
				resultChan <- queuesLoadedMsg{queues: nil, err: err}
				return
			}

			if len(queueURLs) == 0 {
				resultChan <- queuesLoadedMsg{queues: []model.Queue{}, err: nil}
				return
			}

			// Get details for each queue
			var queues []model.Queue
			for _, url := range queueURLs {
				queue, err := m.client.GetQueueAttributes(ctx, url)
				if err != nil {
					continue
				}
				queues = append(queues, *queue)
			}

			// Fetch DLQ message counts
			queues = m.enrichQueuesWithDLQ(ctx, queues)

			resultChan <- queuesLoadedMsg{queues: queues, err: nil}
			return
		}

		// Lazy load with incremental results
		isFirst := true
		err := m.client.ListQueuesPagedCallback(ctx, func(queues []model.Queue, hasMore bool) bool {
			resultChan <- queuesLoadedMsg{
				queues:   queues,
				err:      nil,
				hasMore:  hasMore,
				isAppend: !isFirst,
			}
			isFirst = false
			return true // continue loading
		})
		if err != nil {
			resultChan <- queuesLoadedMsg{queues: nil, err: err}
		}
	}()

	// Return command that reads from channel
	return tea.Batch(
		m.sqsTable.Spinner().TickCmd(),
		func() tea.Msg {
			msg, ok := <-resultChan
			if !ok {
				return nil
			}
			// Store channel for subsequent reads
			m.queuesResultChan = resultChan
			return msg
		},
	)
}

// continueQueuesLoad continues reading from the queues result channel.
func (m *Model) continueQueuesLoad() tea.Cmd {
	if m.queuesResultChan == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-m.queuesResultChan
		if !ok {
			m.queuesResultChan = nil
			return nil
		}
		return msg
	}
}

// enrichQueuesWithDLQ fetches DLQ message counts for queues that have DLQs.
func (m *Model) enrichQueuesWithDLQ(ctx context.Context, queues []model.Queue) []model.Queue {
	// Build ARN -> URL map for DLQ lookups
	dlqURLMap := make(map[string]string)
	for _, q := range queues {
		if q.ARN != "" {
			dlqURLMap[q.ARN] = q.URL
		}
	}

	// Fetch DLQ message counts
	for i := range queues {
		if queues[i].HasDLQ && queues[i].DLQArn != "" {
			dlqURL, ok := dlqURLMap[queues[i].DLQArn]
			if ok {
				out, err := m.client.SQS().GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
					QueueUrl:       &dlqURL,
					AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameApproximateNumberOfMessages},
				})
				if err == nil {
					if countStr, ok := out.Attributes[string(sqstypes.QueueAttributeNameApproximateNumberOfMessages)]; ok {
						count, _ := strconv.Atoi(countStr)
						queues[i].DLQMessageCount = count
						queues[i].DLQURL = dlqURL
						queues[i].DLQName = extractQueueNameFromURL(dlqURL)
					}
				}
			}
		}
	}
	return queues
}

// extractQueueNameFromURL extracts the queue name from a queue URL.
func extractQueueNameFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}

// loadAPIStages loads API stages for the selected API.
func (m *Model) loadAPIStages() tea.Cmd {
	m.state.APIStagesLoading = true
	m.apiStagesList.SetLoading(true)

	var apiID string
	var isRest bool

	if m.state.SelectedRestAPI != nil {
		apiID = m.state.SelectedRestAPI.ID
		isRest = true
		m.logger.Info("Loading stages for REST API: %s", m.state.SelectedRestAPI.Name)
	} else if m.state.SelectedHttpAPI != nil {
		apiID = m.state.SelectedHttpAPI.ID
		isRest = false
		m.logger.Info("Loading stages for HTTP API: %s", m.state.SelectedHttpAPI.Name)
	} else {
		return nil
	}

	return tea.Batch(
		m.apiStagesList.Spinner().TickCmd(),
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			var stages []model.APIStage
			var err error
			if isRest {
				stages, err = m.client.GetRestAPIStages(ctx, apiID)
			} else {
				stages, err = m.client.GetHttpAPIStages(ctx, apiID)
			}
			return apiStagesLoadedMsg{stages: stages, err: err}
		},
	)
}

// loadClusters loads ECS clusters.
func (m *Model) loadClusters() tea.Cmd {
	m.state.ClustersLoading = true
	m.clustersList.SetLoading(true)

	return tea.Batch(
		m.clustersList.Spinner().TickCmd(),
		func() tea.Msg {
			clusters, err := m.client.ListClusters(context.Background())
			if err != nil {
				return errMsg{err: err}
			}
			return clustersLoadedMsg{clusters: clusters}
		},
	)
}

// loadTables loads DynamoDB tables with lazy loading.
func (m *Model) loadTables() tea.Cmd {
	m.state.TablesLoading = true
	m.dynamodbTable.SetLoading(true)
	m.logger.Info("Loading DynamoDB tables...")

	// Use channel for incremental results
	resultChan := make(chan tablesLoadedMsg, 10)

	// Start background loading
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		defer close(resultChan)

		// Lazy load with incremental results
		isFirst := true
		err := m.client.ListTablesPagedCallback(ctx, func(tables []model.Table, hasMore bool) bool {
			resultChan <- tablesLoadedMsg{
				tables:   tables,
				err:      nil,
				hasMore:  hasMore,
				isAppend: !isFirst,
			}
			isFirst = false
			return true // continue loading
		})
		if err != nil {
			resultChan <- tablesLoadedMsg{tables: nil, err: err}
		}
	}()

	// Return command that reads from channel
	return tea.Batch(
		m.dynamodbTable.Spinner().TickCmd(),
		func() tea.Msg {
			msg, ok := <-resultChan
			if !ok {
				return nil
			}
			// Store channel for subsequent reads
			m.tablesResultChan = resultChan
			return msg
		},
	)
}

// continueTablesLoad continues reading from the tables result channel.
func (m *Model) continueTablesLoad() tea.Cmd {
	if m.tablesResultChan == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-m.tablesResultChan
		if !ok {
			m.tablesResultChan = nil
			return nil
		}
		return msg
	}
}

// executeDynamoDBQuery executes a DynamoDB query.
func (m *Model) executeDynamoDBQuery(params *model.QueryParams) tea.Cmd {
	m.state.DynamoDBQueryLoading = true
	m.state.DynamoDBQueryParams = params
	m.state.DynamoDBIsQuery = true
	m.dynamodbQueryResults.SetLoading(true)
	m.logger.Info("Executing DynamoDB query on table: %s", params.TableName)

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := m.client.QueryTable(ctx, *params, m.state.DynamoDBLastKey)
		return dynamoDBQueryResultMsg{result: result, err: err}
	}
}

// executeDynamoDBScan executes a DynamoDB scan.
func (m *Model) executeDynamoDBScan(params *model.ScanParams) tea.Cmd {
	m.state.DynamoDBQueryLoading = true
	m.state.DynamoDBScanParams = params
	m.state.DynamoDBIsQuery = false
	m.dynamodbQueryResults.SetLoading(true)
	m.logger.Info("Executing DynamoDB scan on table: %s", params.TableName)

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := m.client.ScanTable(ctx, *params, m.state.DynamoDBLastKey)
		return dynamoDBQueryResultMsg{result: result, err: err}
	}
}

// loadNextDynamoDBPage loads the next page of DynamoDB results.
func (m *Model) loadNextDynamoDBPage() tea.Cmd {
	if m.state.DynamoDBQueryResult == nil || !m.state.DynamoDBQueryResult.HasMorePages {
		return nil
	}

	m.state.DynamoDBLastKey = m.state.DynamoDBQueryResult.LastEvaluatedKey
	m.state.DynamoDBQueryLoading = true
	m.dynamodbQueryResults.SetLoading(true)

	if m.state.DynamoDBIsQuery && m.state.DynamoDBQueryParams != nil {
		m.logger.Info("Loading next page of query results...")
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			result, err := m.client.QueryTable(ctx, *m.state.DynamoDBQueryParams, m.state.DynamoDBLastKey)
			return dynamoDBQueryResultMsg{result: result, err: err}
		}
	} else if m.state.DynamoDBScanParams != nil {
		m.logger.Info("Loading next page of scan results...")
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			result, err := m.client.ScanTable(ctx, *m.state.DynamoDBScanParams, m.state.DynamoDBLastKey)
			return dynamoDBQueryResultMsg{result: result, err: err}
		}
	}

	return nil
}
