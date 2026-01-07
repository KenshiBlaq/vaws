package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"vaws/internal/model"
	"vaws/internal/state"
)

// updateTunnelsPanel updates the tunnels panel with current tunnel data.
func (m *Model) updateTunnelsPanel() {
	tunnels := m.tunnelManager.GetTunnels()
	var apiGWTunnels []model.APIGatewayTunnel
	if m.apiGWManager != nil {
		apiGWTunnels = m.apiGWManager.GetTunnels()
	}
	m.tunnelsPanel.SetTunnels(tunnels)
	m.tunnelsPanel.SetAPIGatewayTunnels(apiGWTunnels)
}

// startTunnel starts a tunnel with a random local port.
func (m *Model) startTunnel(service model.Service, task model.Task, container model.Container, remotePort int) tea.Cmd {
	return m.startTunnelWithPort(service, task, container, remotePort, 0)
}

// startTunnelWithPort starts a tunnel with a specific local port.
func (m *Model) startTunnelWithPort(service model.Service, task model.Task, container model.Container, remotePort, localPort int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		tunnel, err := m.tunnelManager.StartTunnel(ctx, service, task, container, remotePort, localPort)
		return tunnelStartedMsg{tunnel: tunnel, err: err}
	}
}

// startAPIGatewayTunnel starts a tunnel for the API Gateway based on its type.
func (m *Model) startAPIGatewayTunnel(api interface{}, stage model.APIStage, localPort int) tea.Cmd {
	// Determine if this is a private or public API Gateway
	isPrivate := false
	if restAPI, ok := api.(*model.RestAPI); ok {
		isPrivate = restAPI.EndpointType == "PRIVATE"
	}

	if isPrivate {
		m.logger.Info("Loading EC2 instances for jump host selection...")
		// Store pending tunnel info and show jump host selection
		m.state.PendingTunnelAPI = api
		m.state.PendingTunnelStage = &stage
		m.state.PendingTunnelLocalPort = localPort
		m.state.View = state.ViewJumpHostSelect
		m.state.EC2InstancesLoading = true
		return m.loadEC2Instances()
	}

	// Public API Gateway - start local HTTP proxy
	m.logger.Info("Starting public API Gateway proxy for stage: %s", stage.Name)
	return m.startPublicAPIGWTunnel(api, stage, localPort)
}

// startPublicAPIGWTunnel starts a local HTTP proxy for public API Gateway.
func (m *Model) startPublicAPIGWTunnel(api interface{}, stage model.APIStage, localPort int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		tunnel, err := m.apiGWManager.StartPublicTunnel(ctx, api, stage, localPort)
		return apiGWTunnelStartedMsg{tunnel: tunnel, err: err}
	}
}

// findJumpHostForAPIGateway finds a jump host for private API Gateway access.
func (m *Model) findJumpHostForAPIGateway(api interface{}, stage model.APIStage, localPort int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Get config for the current profile
		jumpHostConfig := ""
		jumpHostTagConfig := ""
		if m.cfg != nil {
			jumpHostConfig = m.cfg.GetJumpHost(m.state.Profile)
			jumpHostTagConfig = m.cfg.GetJumpHostTag(m.state.Profile)
		}

		defaultTags := []string{
			"vaws:jump-host=true",
			"Name=bastion",
			"Name=jump-host",
		}
		defaultNames := []string{
			"bastion",
			"jump-host",
			"jumphost",
		}

		if m.cfg != nil && len(m.cfg.Defaults.JumpHostTags) > 0 {
			defaultTags = m.cfg.Defaults.JumpHostTags
		}
		if m.cfg != nil && len(m.cfg.Defaults.JumpHostNames) > 0 {
			defaultNames = m.cfg.Defaults.JumpHostNames
		}

		// Find jump host
		jumpHost, err := m.client.FindJumpHost(ctx, "", jumpHostConfig, jumpHostTagConfig, defaultTags, defaultNames)
		if err != nil {
			return jumpHostFoundMsg{err: fmt.Errorf("failed to find jump host: %w", err)}
		}

		// Try to find VPC endpoint for execute-api
		var vpcEndpoint *model.VpcEndpoint
		var vpcEndpointErr error
		if jumpHost.VpcID != "" {
			vpcEndpoint, vpcEndpointErr = m.client.FindAPIGatewayVpcEndpoint(ctx, jumpHost.VpcID)
			// Note: vpcEndpointErr is informational - we'll handle missing endpoint in the tunnel manager
		}

		return jumpHostFoundMsg{
			jumpHost:       jumpHost,
			vpcEndpoint:    vpcEndpoint,
			vpcEndpointErr: vpcEndpointErr,
			stage:          stage,
			api:            api,
			localPort:      localPort,
		}
	}
}

// startPrivateAPIGWTunnel starts an SSM tunnel for private API Gateway.
func (m *Model) startPrivateAPIGWTunnel(api interface{}, stage model.APIStage, jumpHost *model.EC2Instance, vpcEndpoint *model.VpcEndpoint, localPort int) tea.Cmd {
	// Get configured VPC endpoint ID for cross-account access
	var configuredVPCEndpointID string
	if m.cfg != nil {
		configuredVPCEndpointID = m.cfg.GetVPCEndpointID(m.state.Profile)
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		tunnel, err := m.apiGWManager.StartPrivateTunnel(ctx, api, stage, jumpHost, vpcEndpoint, configuredVPCEndpointID, localPort)
		return apiGWTunnelStartedMsg{tunnel: tunnel, err: err}
	}
}

// startPrivateAPIGWTunnelWithJumpHost starts a private API Gateway tunnel using the selected jump host.
func (m *Model) startPrivateAPIGWTunnelWithJumpHost(jumpHost *model.EC2Instance) tea.Cmd {
	// Get pending tunnel info
	api := m.state.PendingTunnelAPI
	stage := m.state.PendingTunnelStage
	localPort := m.state.PendingTunnelLocalPort

	// Clear pending tunnel state and go back to stages view
	m.state.ClearPendingTunnel()
	m.state.ClearEC2Instances()
	m.state.View = state.ViewAPIStages
	m.updateAPIStagesList()

	// Get configured VPC endpoint ID for cross-account access
	var configuredVPCEndpointID string
	if m.cfg != nil {
		configuredVPCEndpointID = m.cfg.GetVPCEndpointID(m.state.Profile)
	}

	m.logger.Info("Starting private API Gateway tunnel via jump host: %s", jumpHost.Name)

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Try to find VPC endpoint for execute-api
		var vpcEndpoint *model.VpcEndpoint
		if jumpHost.VpcID != "" {
			vpcEndpoint, _ = m.client.FindAPIGatewayVpcEndpoint(ctx, jumpHost.VpcID)
		}

		tunnel, err := m.apiGWManager.StartPrivateTunnel(ctx, api, *stage, jumpHost, vpcEndpoint, configuredVPCEndpointID, localPort)
		return apiGWTunnelStartedMsg{tunnel: tunnel, err: err}
	}
}
