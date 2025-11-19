package errors

import (
	"fmt"
	"strings"
)

type StorageError struct {
	Operation string
	Err       error
}

func (e *StorageError) Error() string {
	return fmt.Sprintf("storage %s failed: %v", e.Operation, e.Err)
}

func (e *StorageError) Unwrap() error {
	return e.Err
}

func WrapStorage(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &StorageError{
		Operation: operation,
		Err:       err,
	}
}

type ModuleError struct {
	Module    string
	Operation string
	Err       error
}

func (e *ModuleError) Error() string {
	return fmt.Sprintf("module %s: %s failed: %v", e.Module, e.Operation, e.Err)
}

func (e *ModuleError) Unwrap() error {
	return e.Err
}

func WrapModule(module, operation string, err error) error {
	if err == nil {
		return nil
	}
	return &ModuleError{
		Module:    module,
		Operation: operation,
		Err:       err,
	}
}

type InstallError struct {
	Component     string
	File          string
	Err           error
	RecoverySteps []string
}

func (e *InstallError) Error() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Failed to install %s:\n", e.Component))
	if e.File != "" {
		sb.WriteString(fmt.Sprintf("  File: %s\n", e.File))
	}
	if e.Err != nil {
		sb.WriteString(fmt.Sprintf("  Error: %v\n", e.Err))
	}

	if len(e.RecoverySteps) > 0 {
		sb.WriteString("\n  To fix:\n")
		for i, step := range e.RecoverySteps {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, step))
		}
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

func (e *InstallError) Unwrap() error {
	return e.Err
}

func WrapInstall(component, file string, err error, steps ...string) error {
	if err == nil {
		return nil
	}
	return &InstallError{
		Component:     component,
		File:          file,
		Err:           err,
		RecoverySteps: steps,
	}
}

type DaemonError struct {
	Component string
	Err       error
}

func (e *DaemonError) Error() string {
	return fmt.Sprintf("daemon %s: %v", e.Component, e.Err)
}

func (e *DaemonError) Unwrap() error {
	return e.Err
}

func WrapDaemon(component string, err error) error {
	if err == nil {
		return nil
	}
	return &DaemonError{
		Component: component,
		Err:       err,
	}
}

type PluginError struct {
	Plugin    string
	Operation string
	Err       error
}

func (e *PluginError) Error() string {
	return fmt.Sprintf("plugin %s: %s failed: %v", e.Plugin, e.Operation, e.Err)
}

func (e *PluginError) Unwrap() error {
	return e.Err
}

func WrapPlugin(plugin, operation string, err error) error {
	if err == nil {
		return nil
	}
	return &PluginError{
		Plugin:    plugin,
		Operation: operation,
		Err:       err,
	}
}

type QueueError struct {
	Operation string
	Err       error
}

func (e *QueueError) Error() string {
	return fmt.Sprintf("queue %s: %v", e.Operation, e.Err)
}

func (e *QueueError) Unwrap() error {
	return e.Err
}

func WrapQueue(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &QueueError{
		Operation: operation,
		Err:       err,
	}
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation failed: %s", e.Message)
}

func NewValidation(field, message string) error {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
