package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestWrapStorage(t *testing.T) {
	t.Run("wraps error with operation context", func(t *testing.T) {
		baseErr := errors.New("connection failed")
		err := WrapStorage("open database", baseErr)

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		expected := "storage open database failed: connection failed"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		err := WrapStorage("some operation", nil)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("unwraps to original error", func(t *testing.T) {
		baseErr := errors.New("original error")
		wrapped := WrapStorage("query", baseErr)

		unwrapped := errors.Unwrap(wrapped)
		if unwrapped != baseErr {
			t.Errorf("expected unwrapped error to be %v, got %v", baseErr, unwrapped)
		}
	})

	t.Run("can be checked with errors.Is", func(t *testing.T) {
		baseErr := errors.New("specific error")
		wrapped := WrapStorage("operation", baseErr)

		if !errors.Is(wrapped, baseErr) {
			t.Error("expected errors.Is to find base error")
		}
	})
}

func TestWrapModule(t *testing.T) {
	t.Run("wraps error with module and operation context", func(t *testing.T) {
		baseErr := errors.New("hook failed")
		err := WrapModule("git", "install", baseErr)

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		expected := "module git: install failed: hook failed"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		err := WrapModule("shell", "validate", nil)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("unwraps to original error", func(t *testing.T) {
		baseErr := fmt.Errorf("file not found")
		wrapped := WrapModule("tmux", "configure", baseErr)

		unwrapped := errors.Unwrap(wrapped)
		if unwrapped != baseErr {
			t.Errorf("expected unwrapped error to be %v, got %v", baseErr, unwrapped)
		}
	})
}

func TestWrapInstall(t *testing.T) {
	t.Run("creates error with component and file", func(t *testing.T) {
		baseErr := errors.New("permission denied")
		err := WrapInstall("git wrapper", "/usr/local/bin/git", baseErr)

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		msg := err.Error()
		if !strings.Contains(msg, "Failed to install git wrapper") {
			t.Errorf("expected component in message, got %q", msg)
		}
		if !strings.Contains(msg, "/usr/local/bin/git") {
			t.Errorf("expected file path in message, got %q", msg)
		}
		if !strings.Contains(msg, "permission denied") {
			t.Errorf("expected base error in message, got %q", msg)
		}
	})

	t.Run("includes recovery steps", func(t *testing.T) {
		baseErr := errors.New("write failed")
		steps := []string{
			"Check file permissions",
			"Ensure directory exists",
			"Try manual install",
		}
		err := WrapInstall("shell integration", "~/.bashrc", baseErr, steps...)

		msg := err.Error()
		if !strings.Contains(msg, "To fix:") {
			t.Errorf("expected recovery steps section, got %q", msg)
		}
		for i, step := range steps {
			expected := fmt.Sprintf("%d. %s", i+1, step)
			if !strings.Contains(msg, expected) {
				t.Errorf("expected step %q in message, got %q", expected, msg)
			}
		}
	})

	t.Run("works without file path", func(t *testing.T) {
		baseErr := errors.New("dependency missing")
		err := WrapInstall("kubectl wrapper", "", baseErr)

		msg := err.Error()
		if !strings.Contains(msg, "Failed to install kubectl wrapper") {
			t.Errorf("expected component in message, got %q", msg)
		}
		if !strings.Contains(msg, "dependency missing") {
			t.Errorf("expected base error in message, got %q", msg)
		}
	})

	t.Run("works without recovery steps", func(t *testing.T) {
		baseErr := errors.New("unknown error")
		err := WrapInstall("component", "/path/to/file", baseErr)

		msg := err.Error()
		if strings.Contains(msg, "To fix:") {
			t.Errorf("expected no recovery steps section, got %q", msg)
		}
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		err := WrapInstall("component", "/path", nil, "step1", "step2")
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("unwraps to original error", func(t *testing.T) {
		baseErr := errors.New("base error")
		wrapped := WrapInstall("component", "/path", baseErr, "step1")

		unwrapped := errors.Unwrap(wrapped)
		if unwrapped != baseErr {
			t.Errorf("expected unwrapped error to be %v, got %v", baseErr, unwrapped)
		}
	})
}

func TestWrapDaemon(t *testing.T) {
	t.Run("wraps error with daemon component context", func(t *testing.T) {
		baseErr := errors.New("startup failed")
		err := WrapDaemon("plugin manager", baseErr)

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		expected := "daemon plugin manager: startup failed"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		err := WrapDaemon("api server", nil)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("unwraps to original error", func(t *testing.T) {
		baseErr := errors.New("port already in use")
		wrapped := WrapDaemon("http server", baseErr)

		unwrapped := errors.Unwrap(wrapped)
		if unwrapped != baseErr {
			t.Errorf("expected unwrapped error to be %v, got %v", baseErr, unwrapped)
		}
	})
}

func TestWrapPlugin(t *testing.T) {
	t.Run("wraps error with plugin and operation context", func(t *testing.T) {
		baseErr := errors.New("API timeout")
		err := WrapPlugin("llm", "query", baseErr)

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		expected := "plugin llm: query failed: API timeout"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		err := WrapPlugin("summarizer", "process", nil)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("unwraps to original error", func(t *testing.T) {
		baseErr := errors.New("config invalid")
		wrapped := WrapPlugin("query", "start", baseErr)

		unwrapped := errors.Unwrap(wrapped)
		if unwrapped != baseErr {
			t.Errorf("expected unwrapped error to be %v, got %v", baseErr, unwrapped)
		}
	})
}

func TestWrapQueue(t *testing.T) {
	t.Run("wraps error with queue operation context", func(t *testing.T) {
		baseErr := errors.New("disk full")
		err := WrapQueue("enqueue", baseErr)

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		expected := "queue enqueue: disk full"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		err := WrapQueue("dequeue", nil)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("unwraps to original error", func(t *testing.T) {
		baseErr := errors.New("file corrupt")
		wrapped := WrapQueue("list", baseErr)

		unwrapped := errors.Unwrap(wrapped)
		if unwrapped != baseErr {
			t.Errorf("expected unwrapped error to be %v, got %v", baseErr, unwrapped)
		}
	})
}

func TestNewValidation(t *testing.T) {
	t.Run("creates validation error with field", func(t *testing.T) {
		err := NewValidation("email", "must be valid email address")

		expected := "validation failed for email: must be valid email address"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("creates validation error without field", func(t *testing.T) {
		err := NewValidation("", "request body is required")

		expected := "validation failed: request body is required"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})

	t.Run("handles empty message", func(t *testing.T) {
		err := NewValidation("field", "")

		expected := "validation failed for field: "
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})
}

func TestErrorChaining(t *testing.T) {
	t.Run("multiple layers of wrapping", func(t *testing.T) {
		baseErr := errors.New("disk read error")
		storageErr := WrapStorage("query events", baseErr)
		moduleErr := WrapModule("git", "poll", storageErr)

		if !errors.Is(moduleErr, baseErr) {
			t.Error("expected errors.Is to find base error through multiple wraps")
		}

		msg := moduleErr.Error()
		if !strings.Contains(msg, "module git") {
			t.Errorf("expected module context in error message, got %q", msg)
		}
	})

	t.Run("can be used with errors.As", func(t *testing.T) {
		baseErr := errors.New("some error")
		wrapped := WrapStorage("operation", baseErr)

		var storageErr *StorageError
		if !errors.As(wrapped, &storageErr) {
			t.Error("expected errors.As to extract StorageError")
		}

		if storageErr.Operation != "operation" {
			t.Errorf("expected operation %q, got %q", "operation", storageErr.Operation)
		}
	})
}

func TestErrorTypes(t *testing.T) {
	t.Run("StorageError type assertion", func(t *testing.T) {
		err := WrapStorage("test", errors.New("err"))
		if _, ok := err.(*StorageError); !ok {
			t.Error("expected StorageError type")
		}
	})

	t.Run("ModuleError type assertion", func(t *testing.T) {
		err := WrapModule("mod", "op", errors.New("err"))
		if _, ok := err.(*ModuleError); !ok {
			t.Error("expected ModuleError type")
		}
	})

	t.Run("InstallError type assertion", func(t *testing.T) {
		err := WrapInstall("comp", "file", errors.New("err"))
		if _, ok := err.(*InstallError); !ok {
			t.Error("expected InstallError type")
		}
	})

	t.Run("DaemonError type assertion", func(t *testing.T) {
		err := WrapDaemon("comp", errors.New("err"))
		if _, ok := err.(*DaemonError); !ok {
			t.Error("expected DaemonError type")
		}
	})

	t.Run("PluginError type assertion", func(t *testing.T) {
		err := WrapPlugin("plugin", "op", errors.New("err"))
		if _, ok := err.(*PluginError); !ok {
			t.Error("expected PluginError type")
		}
	})

	t.Run("QueueError type assertion", func(t *testing.T) {
		err := WrapQueue("op", errors.New("err"))
		if _, ok := err.(*QueueError); !ok {
			t.Error("expected QueueError type")
		}
	})

	t.Run("ValidationError type assertion", func(t *testing.T) {
		err := NewValidation("field", "msg")
		if _, ok := err.(*ValidationError); !ok {
			t.Error("expected ValidationError type")
		}
	})
}
