package profile

import "fmt"

// Profile defines resource limits for cluster nodes.
type Profile struct {
	Name        string
	CPU         float64
	Memory      int64
	MaxPods     int
	Description string
}

var builtinProfiles = map[string]Profile{
	"light": {
		Name:        "light",
		CPU:         0.5,
		Memory:      512 * 1024 * 1024,
		MaxPods:     5,
		Description: "Laptop with limited resources",
	},
	"standard": {
		Name:        "standard",
		CPU:         1.0,
		Memory:      1024 * 1024 * 1024,
		MaxPods:     15,
		Description: "Standard development",
	},
	"heavy": {
		Name:        "heavy",
		CPU:         2.0,
		Memory:      2 * 1024 * 1024 * 1024,
		MaxPods:     30,
		Description: "Workstation / CI",
	},
}

// Get returns a profile by name.
func Get(name string) (Profile, error) {
	p, ok := builtinProfiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("unknown profile %q: available profiles are light, standard, heavy", name)
	}
	return p, nil
}

// List returns all available profiles.
func List() []Profile {
	result := make([]Profile, 0, len(builtinProfiles))
	for _, p := range builtinProfiles {
		result = append(result, p)
	}
	return result
}

// Default returns the default profile name.
func Default() string {
	return "standard"
}
