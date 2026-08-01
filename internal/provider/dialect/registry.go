package dialect

import (
	"fmt"
	"strings"
)

type Registry struct {
	dialects map[string]Dialect
}

func NewRegistry() *Registry {
	return &Registry{dialects: make(map[string]Dialect)}
}

func (r *Registry) Register(value Dialect) error {
	if r == nil || value == nil {
		return fmt.Errorf("dialect is required")
	}
	name := value.Name()
	if strings.TrimSpace(name) == "" || name != strings.TrimSpace(name) {
		return fmt.Errorf("dialect name is invalid")
	}
	if _, exists := r.dialects[name]; exists {
		return fmt.Errorf("dialect %q is already registered", name)
	}
	r.dialects[name] = value
	return nil
}

func (r *Registry) Get(name string) (Dialect, error) {
	if r == nil {
		return nil, fmt.Errorf("dialect registry is unavailable")
	}
	value, ok := r.dialects[name]
	if !ok {
		return nil, fmt.Errorf("dialect %q is not registered", name)
	}
	return value, nil
}
