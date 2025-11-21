package kubectl

import (
	"fmt"
	"strings"

	"devlog/internal/events"
	"devlog/internal/formatting"
)

type KubectlFormatter struct{}

func init() {
	formatting.Register("kubectl", &KubectlFormatter{})
}

func (f *KubectlFormatter) Format(event *events.Event) string {
	operation := strings.TrimPrefix(event.Type, "kubectl_")

	resourceType := ""
	if rt, ok := event.Payload["resource_type"].(string); ok && rt != "" {
		resourceType = rt
	}

	resourceNames := ""
	if rn, ok := event.Payload["resource_names"].(string); ok && rn != "" {
		resourceNames = rn
	}

	namespace := ""
	if ns, ok := event.Payload["namespace"].(string); ok && ns != "" {
		namespace = ns
	}

	context := ""
	if ctx, ok := event.Payload["context"].(string); ok && ctx != "" {
		context = ctx
	}

	exitCode := 0
	if ec, ok := event.Payload["exit_code"].(float64); ok {
		exitCode = int(ec)
	}

	var parts []string
	parts = append(parts, operation)

	if resourceType != "" {
		if resourceNames != "" {
			parts = append(parts, fmt.Sprintf("%s/%s", resourceType, resourceNames))
		} else {
			parts = append(parts, resourceType)
		}
	}

	if namespace != "" {
		parts = append(parts, fmt.Sprintf("-n %s", namespace))
	}

	if context != "" {
		parts = append(parts, fmt.Sprintf("@%s", context))
	}

	result := strings.Join(parts, " ")

	if exitCode != 0 {
		result += fmt.Sprintf(" [exit:%d]", exitCode)
	}

	return result
}
