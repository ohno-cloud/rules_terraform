package atlantis

import "strings"

type MultiEnv struct {
	environ map[string]string
}

// Add returns true if it overrides a value
func (m *MultiEnv) Add(name, value string) bool {
	_, ok := m.environ[name]
	m.environ[name] = value
	return ok
}

func (m *MultiEnv) Output() string {
	pairs := make([]string, len(m.environ))

	for key, val := range m.environ {
		pairs = append(pairs, key+"="+val)
	}

	return strings.Join(pairs, ",")
}
