package client

import (
	"context"
	"errors"
	"reflect"
	"strings"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
)

// Reconfigure 对运行中的会话应用可热更新配置；不可热更新时返回 ErrRestartRequired。
func (s *Session) Reconfigure(ctx context.Context, options Options) error {
	core, err := s.activeCore()
	if err != nil {
		return err
	}
	return core.reconfigure(ctx, options)
}

func (c *sessionCore) reconfigure(ctx context.Context, options Options) error {
	if !c.isConnected() {
		return ErrNotConnected
	}

	nextOptions, err := options.normalized()
	if err != nil {
		return err
	}
	currentOptions := c.options

	if reason, ok := restartReasonForReconfigure(currentOptions, nextOptions); ok {
		return &RestartRequiredError{Reason: reason}
	}
	if err := c.applyRuntimeReconfigure(ctx, currentOptions, nextOptions); err != nil {
		if isMCPSetServersUnsupported(err) {
			return &RestartRequiredError{
				Reason: RestartReasonMCPControlUnsupported,
				Cause:  err,
			}
		}
		return err
	}
	c.options = nextOptions
	return nil
}

func restartReasonForReconfigure(currentOptions Options, nextOptions Options) (RestartReason, bool) {
	if !stringMapsEqual(currentOptions.Env, nextOptions.Env) && normalizedRuntimeKind(nextOptions.Runtime.Kind) != RuntimeNXS {
		return RestartReasonProcessEnvChanged, true
	}
	if !reflect.DeepEqual(currentOptions.Tools.Allow, nextOptions.Tools.Allow) ||
		!reflect.DeepEqual(currentOptions.Tools.Deny, nextOptions.Tools.Deny) {
		return RestartReasonToolPolicyChanged, true
	}
	if !reflect.DeepEqual(currentOptions.Skills, nextOptions.Skills) ||
		!reflect.DeepEqual(currentOptions.AdditionalDirectories, nextOptions.AdditionalDirectories) ||
		!reflect.DeepEqual(currentOptions.SettingSources, nextOptions.SettingSources) {
		return RestartReasonSkillConfigChanged, true
	}
	return "", false
}

func (c *sessionCore) applyRuntimeReconfigure(
	ctx context.Context,
	currentOptions Options,
	nextOptions Options,
) error {
	if !stringMapsEqual(currentOptions.Env, nextOptions.Env) {
		if err := c.updateEnvironment(ctx, environmentDelta(currentOptions.Env, nextOptions.Env)); err != nil {
			return err
		}
	}
	if nextOptions.Runtime.PermissionMode != "" &&
		nextOptions.Runtime.PermissionMode != currentOptions.Runtime.PermissionMode {
		if err := c.setPermissionMode(ctx, nextOptions.Runtime.PermissionMode); err != nil {
			return err
		}
	}

	nextModel := strings.TrimSpace(nextOptions.Model)
	currentModel := strings.TrimSpace(currentOptions.Model)
	if nextModel != "" && nextModel != currentModel {
		if err := c.setModel(ctx, nextModel); err != nil {
			return err
		}
	}

	nextMaxThinkingTokens := nextOptions.Runtime.MaxThinkingTokens
	if nextMaxThinkingTokens > 0 &&
		nextMaxThinkingTokens != currentOptions.Runtime.MaxThinkingTokens {
		if err := c.setMaxThinkingTokens(ctx, nextMaxThinkingTokens); err != nil {
			return err
		}
	}
	if shouldSyncMCPServersForRuntimeReconfigure(currentOptions, nextOptions) {
		if _, err := c.setMCPServers(ctx, resolvedMCPServersForRuntimeReconfigure(nextOptions)); err != nil {
			return err
		}
	}
	return nil
}

func environmentDelta(current map[string]string, next map[string]string) map[string]string {
	delta := make(map[string]string)
	for key, value := range next {
		if current[key] != value {
			delta[key] = value
		}
	}
	for key := range current {
		if _, exists := next[key]; !exists {
			delta[key] = ""
		}
	}
	return delta
}

func shouldSyncMCPServersForRuntimeReconfigure(currentOptions Options, nextOptions Options) bool {
	return !reflect.DeepEqual(
		resolvedMCPServersForRuntimeReconfigure(currentOptions),
		resolvedMCPServersForRuntimeReconfigure(nextOptions),
	)
}

func resolvedMCPServersForRuntimeReconfigure(options Options) map[string]mcp.ServerConfig {
	return options.resolvedMCPServers()
}

func stringMapsEqual(left map[string]string, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, leftValue := range left {
		rightValue, ok := right[key]
		if !ok || rightValue != leftValue {
			return false
		}
	}
	return true
}

func isMCPSetServersUnsupported(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if !(strings.Contains(message, "mcp_set_servers") || strings.Contains(message, "mcp set servers")) {
		return false
	}
	return errors.Is(err, ErrUnsupportedCapability) ||
		strings.Contains(message, "unsupported") ||
		strings.Contains(message, "not supported") ||
		strings.Contains(message, "unknown")
}
