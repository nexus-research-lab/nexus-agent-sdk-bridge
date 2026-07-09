package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/agent"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/jsonvalue"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/transport"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func decodePermissionUpdates(raw any) []permission.Update {
	items := jsonvalue.SliceValue(raw)
	updates := make([]permission.Update, 0, len(items))
	for _, item := range items {
		payload := jsonvalue.MapValue(item)
		if len(payload) == 0 {
			continue
		}

		updates = append(updates, permission.Update{
			Type:        jsonvalue.StringValue(payload["type"]),
			Rules:       decodePermissionRuleValues(payload["rules"]),
			Behavior:    permission.Behavior(jsonvalue.StringValue(payload["behavior"])),
			Mode:        permission.Mode(jsonvalue.StringValue(payload["mode"])),
			Directories: jsonvalue.StringSliceValue(payload["directories"]),
			Destination: permission.UpdateDestination(jsonvalue.StringValue(payload["destination"])),
		})
	}
	return updates
}

func decodePermissionRuleValues(raw any) []permission.RuleValue {
	items := jsonvalue.SliceValue(raw)
	values := make([]permission.RuleValue, 0, len(items))
	for _, item := range items {
		payload := jsonvalue.MapValue(item)
		if len(payload) == 0 {
			continue
		}
		values = append(values, permission.RuleValue{
			ToolName:    jsonvalue.StringValue(payload["tool_name"]),
			RuleContent: jsonvalue.StringValue(payload["rule_content"]),
		})
	}
	return values
}
func (c *sessionCore) resolvePermissionRequest(ctx context.Context, request map[string]any) map[string]any {
	if c.options.Callbacks.PermissionHandler == nil {
		return map[string]any{
			"behavior": "deny",
			"message":  "permission handler is not configured",
		}
	}

	permissionRequest := permission.Request{
		ToolName:              jsonvalue.StringValue(request["tool_name"]),
		Input:                 jsonvalue.MapValue(request["input"]),
		PermissionSuggestions: decodePermissionUpdates(request["permission_suggestions"]),
		BlockedPath:           jsonvalue.StringValue(request["blocked_path"]),
		DecisionReason:        jsonvalue.StringValue(request["decision_reason"]),
		Title:                 jsonvalue.StringValue(request["title"]),
		DisplayName:           jsonvalue.StringValue(request["display_name"]),
		Description:           jsonvalue.StringValue(request["description"]),
		ToolUseID:             jsonvalue.StringValue(request["tool_use_id"]),
		AgentID:               jsonvalue.StringValue(request["agent_id"]),
	}

	decision, err := c.options.Callbacks.PermissionHandler(ctx, permissionRequest)
	if err != nil {
		return map[string]any{
			"behavior": "deny",
			"message":  err.Error(),
		}
	}

	if decision.Behavior == permission.BehaviorAllow {
		updatedInput := decision.UpdatedInput
		if updatedInput == nil {
			updatedInput = permissionRequest.Input
		}
		response := map[string]any{
			"behavior":      "allow",
			"updated_input": updatedInput,
		}
		if len(decision.UpdatedPermissions) > 0 {
			response["updated_permissions"] = append([]permission.Update(nil), decision.UpdatedPermissions...)
		}
		return response
	}

	response := map[string]any{
		"behavior": "deny",
		"message":  decision.Message,
	}
	if decision.Interrupt {
		response["interrupt"] = true
	}
	return response
}

func (c *sessionCore) resolveHookCallback(ctx context.Context, request map[string]any) (map[string]any, error) {
	callbackID := jsonvalue.StringValue(request["callback_id"])
	if callbackID == "" {
		return nil, errors.New("hook callback id is missing")
	}

	callback, ok := c.hookCallbackRegistry().get(callbackID)
	if !ok {
		return nil, fmt.Errorf("hook callback not found: %s", callbackID)
	}

	input := jsonvalue.MapValue(request["input"])
	output, err := callback(ctx, hook.NewInput(input), jsonvalue.StringValue(request["tool_use_id"]))
	if err != nil {
		return nil, err
	}
	return output.ToMap(), nil
}

func (c *sessionCore) resolveElicitation(ctx context.Context, request map[string]any) (map[string]any, error) {
	elicitationRequest := protocol.DecodeElicitationRequest(request)

	if c.options.Callbacks.ElicitationHandler == nil {
		return protocol.ElicitationResponse{Action: "decline"}.ContentMap(), nil
	}
	response, err := c.options.Callbacks.ElicitationHandler(ctx, elicitationRequest)
	if err != nil {
		return nil, err
	}
	return protocol.NormalizeElicitationResponse(response).ContentMap(), nil
}

func (c *sessionCore) resolveUserDialog(ctx context.Context, request map[string]any) (map[string]any, error) {
	if c.options.Callbacks.UserDialogHandler == nil {
		return nil, errors.New("user dialog handler is not configured")
	}

	response, err := c.options.Callbacks.UserDialogHandler(ctx, protocol.DecodeUserDialogRequest(request))
	if err != nil {
		return nil, err
	}
	return response.ContentMap(), nil
}

func (c *sessionCore) resolveOAuthTokenRefresh(ctx context.Context) (map[string]any, error) {
	if c.options.Callbacks.OAuthTokenHandler == nil {
		return nil, errors.New("oauth token handler is not configured")
	}

	accessToken, err := c.options.Callbacks.OAuthTokenHandler(ctx)
	if err != nil {
		return nil, err
	}
	if accessToken == "" {
		return map[string]any{
			"accessToken": nil,
		}, nil
	}
	return map[string]any{
		"accessToken": accessToken,
	}, nil
}
func (c *sessionCore) buildInitializeRequest() protocol.ControlRequest {
	request := protocol.ControlRequest{
		Subtype: "initialize",
	}

	if hooks := c.buildHookInitialization(); len(hooks) > 0 {
		request.Hooks = hooks
	}
	if len(c.options.Agents) > 0 {
		request.Agents = agent.EncodeDefinitions(c.options.Agents)
	}
	if registry := c.currentSDKMCPServers(); len(registry) > 0 {
		request.SDKMCPServers = sortedKeys(registry)
	}
	if schema := c.options.outputSchema(); len(schema) > 0 {
		request.JSONSchema = schema
	}
	if skills := c.options.Skills.controlValue(); skills != nil {
		request.Skills = skills
	}
	if c.options.System.Text != "" {
		request.SystemPrompt = c.options.System.Text
	}
	if c.options.System.Append != "" {
		request.AppendSystemPrompt = c.options.System.Append
	}
	if c.options.System.ExcludeDynamicSections != nil {
		request.ExcludeDynamicSections = c.options.System.ExcludeDynamicSections
	}
	if c.options.System.AgentProgressSummaries != nil {
		request.AgentProgressSummaries = c.options.System.AgentProgressSummaries
	}
	return request
}

func (c *sessionCore) buildHookInitialization() map[string]any {
	result := map[string]any{}
	if len(c.options.Hooks.Matchers) == 0 {
		return result
	}

	for event, matchers := range c.options.Hooks.Matchers {
		if len(matchers) == 0 {
			continue
		}

		encodedMatchers := make([]map[string]any, 0, len(matchers))
		for _, matcher := range matchers {
			callbackIDs := make([]string, 0, len(matcher.Hooks))
			for _, callback := range matcher.Hooks {
				callbackID := c.hookCallbackRegistry().register(callback)
				if callbackID == "" {
					continue
				}
				callbackIDs = append(callbackIDs, callbackID)
			}
			if len(callbackIDs) == 0 {
				continue
			}

			encoded := map[string]any{
				"hook_callback_ids": callbackIDs,
			}
			if matcher.Matcher != "" {
				encoded["matcher"] = matcher.Matcher
			}
			if matcher.Timeout > 0 {
				encoded["timeout"] = matcher.Timeout.Seconds()
			}
			encodedMatchers = append(encodedMatchers, encoded)
		}

		if len(encodedMatchers) > 0 {
			result[string(event)] = encodedMatchers
		}
	}

	return result
}

func (c *sessionCore) handleControlResponse(payload map[string]any) {
	response := jsonvalue.MapValue(payload["response"])
	requestID := jsonvalue.StringValue(response["request_id"])
	if requestID == "" {
		return
	}

	subtype := jsonvalue.StringValue(response["subtype"])
	if subtype == "error" {
		c.pendingRequests.resolve(requestID, controlWaitResult{Err: errors.New(jsonvalue.StringValue(response["error"]))})
		return
	}

	c.pendingRequests.resolve(requestID, controlWaitResult{Response: jsonvalue.MapValue(response["response"])})
}

func (c *sessionCore) handleControlRequest(payload map[string]any) {
	requestID := jsonvalue.StringValue(payload["request_id"])
	request := jsonvalue.MapValue(payload["request"])
	if requestID == "" || len(request) == 0 {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.inflightRequests.add(requestID, cancel)
	defer func() {
		c.inflightRequests.remove(requestID)
		cancel()
	}()

	subtype := jsonvalue.StringValue(request["subtype"])
	switch subtype {
	case "can_use_tool":
		response := c.resolvePermissionRequest(ctx, request)
		if ctx.Err() != nil {
			return
		}
		c.writeControlResponse(protocol.NewControlSuccessResponse(requestID, response))
	case "hook_callback":
		response, err := c.resolveHookCallback(ctx, request)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			c.writeControlResponse(protocol.NewControlErrorResponse(requestID, err.Error()))
			return
		}
		c.writeControlResponse(protocol.NewControlSuccessResponse(requestID, response))
	case "mcp_message":
		response, err := c.resolveMCPMessage(ctx, request)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			c.writeControlResponse(protocol.NewControlErrorResponse(requestID, err.Error()))
			return
		}
		c.writeControlResponse(protocol.NewControlSuccessResponse(requestID, map[string]any{
			"mcp_response": response,
		}))
	case "elicitation":
		response, err := c.resolveElicitation(ctx, request)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			c.writeControlResponse(protocol.NewControlErrorResponse(requestID, err.Error()))
			return
		}
		c.writeControlResponse(protocol.NewControlSuccessResponse(requestID, response))
	case "request_user_dialog":
		response, err := c.resolveUserDialog(ctx, request)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			c.writeControlResponse(protocol.NewControlErrorResponse(requestID, err.Error()))
			return
		}
		c.writeControlResponse(protocol.NewControlSuccessResponse(requestID, response))
	case "oauth_token_refresh":
		response, err := c.resolveOAuthTokenRefresh(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			c.writeControlResponse(protocol.NewControlErrorResponse(requestID, err.Error()))
			return
		}
		c.writeControlResponse(protocol.NewControlSuccessResponse(requestID, response))
	default:
		c.writeControlResponse(
			protocol.NewControlErrorResponse(requestID, fmt.Sprintf("unsupported control request subtype: %s", subtype)),
		)
	}
}

func (c *sessionCore) writeControlResponse(payload any) {
	if c.transport == nil {
		c.markTransportFailed(ErrNotConnected)
		return
	}
	if err := c.transport.WriteJSON(payload); err != nil {
		c.markTransportFailed(fmt.Errorf("client: send control response failed: %w", err))
	}
}

func (c *sessionCore) handleControlCancelRequest(payload map[string]any) {
	requestID := jsonvalue.StringValue(payload["request_id"])
	if requestID == "" {
		return
	}

	c.inflightRequests.cancel(requestID)
}

func (c *sessionCore) sendControlRequest(
	ctx context.Context,
	request protocol.ControlRequest,
	timeout time.Duration,
) (map[string]any, error) {
	if c.transport == nil {
		return nil, ErrNotConnected
	}

	requestID := c.pendingRequests.nextID()
	waiter := c.pendingRequests.register(requestID)

	if err := c.transport.WriteJSON(protocol.NewControlRequestEnvelope(requestID, request)); err != nil {
		c.pendingRequests.delete(requestID)
		if transport.IsTransportWriteFailure(err) {
			c.markTransportFailed(fmt.Errorf("client: send control request failed: %w", err))
		}
		return nil, fmt.Errorf("client: send control request failed: %w", err)
	}

	waitContext := ctx
	cancel := func() {}
	if timeout > 0 {
		waitContext, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	select {
	case result := <-waiter:
		return result.Response, result.Err
	case <-waitContext.Done():
		c.pendingRequests.delete(requestID)
		return nil, waitContext.Err()
	}
}

func (c *sessionCore) failPendingRequests(err error) {
	c.pendingRequests.rejectAll(err)
}

func (c *sessionCore) setReadError(err error) {
	c.lifecycleState().setReadError(err)
}

func (c *sessionCore) getReadError() error {
	return c.lifecycleState().readError()
}
