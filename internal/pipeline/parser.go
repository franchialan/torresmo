package pipeline

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

// Torresmofile represents a parsed Torresmofile.
type Torresmofile struct {
	Version string   `hcl:"version"`
	Bases   []Base   `hcl:"base,block"`
	Targets []Target `hcl:"target,block"`
}

// Base defines a reusable base image configuration.
type Base struct {
	Name    string `hcl:"name,label"`
	Image   string `hcl:"image"`
	Workdir string `hcl:"workdir,optional"`
}

// Target defines a pipeline target (step).
type Target struct {
	Name         string         `hcl:"name,label"`
	From         string         `hcl:"from,optional"`
	FromImage    string         `hcl:"from_image,optional"`
	DependsOn    []string       `hcl:"depends_on,optional"`
	NeedsCluster bool           `hcl:"needs_cluster,optional"`
	Addons       []string       `hcl:"addons,optional"`
	Copies       []Copy         `hcl:"copy,block"`
	Runs         []Run          `hcl:"run,block"`
	Artifacts    []Artifact     `hcl:"artifact,block"`
	CopyArtifacts []CopyArtifact `hcl:"copy_artifact,block"`
	EnvBlocks    []EnvBlock     `hcl:"env,block"`
}

// Copy defines a file copy from host to container.
type Copy struct {
	Src  string `hcl:"src"`
	Dest string `hcl:"dest"`
}

// Run defines a command to execute.
type Run struct {
	Command string `hcl:"command,label"`
}

// Artifact defines an output artifact.
type Artifact struct {
	Path string `hcl:"path"`
}

// CopyArtifact copies an artifact from another target.
type CopyArtifact struct {
	From string `hcl:"from"`
	Src  string `hcl:"src"`
	Dest string `hcl:"dest"`
}

// EnvBlock defines environment variables using HCL remain to capture arbitrary keys.
type EnvBlock struct {
	Remain hcl.Body `hcl:",remain"`
}

// ParseFile parses a Torresmofile from the given path.
func ParseFile(path string) (*Torresmofile, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return Parse(path, src)
}

// Parse parses HCL content into a Torresmofile.
func Parse(filename string, src []byte) (*Torresmofile, error) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(src, filename)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parsing %s: %s", filename, diags.Error())
	}

	var tf Torresmofile
	diags = gohcl.DecodeBody(file.Body, nil, &tf)
	if diags.HasErrors() {
		return nil, fmt.Errorf("decoding %s: %s", filename, diags.Error())
	}

	if err := validate(&tf); err != nil {
		return nil, fmt.Errorf("validating %s: %w", filename, err)
	}

	return &tf, nil
}

func validate(tf *Torresmofile) error {
	if tf.Version == "" {
		return fmt.Errorf("version is required")
	}

	bases := make(map[string]struct{}, len(tf.Bases))
	for _, b := range tf.Bases {
		bases[b.Name] = struct{}{}
	}

	targets := make(map[string]struct{}, len(tf.Targets))
	for _, t := range tf.Targets {
		if _, dup := targets[t.Name]; dup {
			return fmt.Errorf("duplicate target %q", t.Name)
		}
		targets[t.Name] = struct{}{}
	}

	for _, t := range tf.Targets {
		if t.From == "" && t.FromImage == "" {
			return fmt.Errorf("target %q must have either 'from' or 'from_image'", t.Name)
		}
		if t.From != "" {
			if _, ok := bases[t.From]; !ok {
				return fmt.Errorf("target %q references undefined base %q", t.Name, t.From)
			}
		}
		for _, dep := range t.DependsOn {
			if _, ok := targets[dep]; !ok {
				return fmt.Errorf("target %q depends on undefined target %q", t.Name, dep)
			}
		}
	}

	if err := checkCycles(tf.Targets); err != nil {
		return err
	}

	return nil
}

func checkCycles(targets []Target) error {
	adj := make(map[string][]string, len(targets))
	for _, t := range targets {
		adj[t.Name] = t.DependsOn
	}

	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)
	state := make(map[string]int, len(targets))

	var visit func(name string, path []string) error
	visit = func(name string, path []string) error {
		switch state[name] {
		case visiting:
			return fmt.Errorf("circular dependency: %s", formatCycle(path, name))
		case visited:
			return nil
		}
		state[name] = visiting
		path = append(path, name)
		for _, dep := range adj[name] {
			if err := visit(dep, path); err != nil {
				return err
			}
		}
		state[name] = visited
		return nil
	}

	for _, t := range targets {
		if state[t.Name] == unvisited {
			if err := visit(t.Name, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func formatCycle(path []string, back string) string {
	s := ""
	found := false
	for _, p := range path {
		if p == back {
			found = true
		}
		if found {
			s += p + " -> "
		}
	}
	s += back
	return s
}

// ResolveImage returns the Docker image for a target.
func (tf *Torresmofile) ResolveImage(target *Target) (string, error) {
	if target.FromImage != "" {
		return target.FromImage, nil
	}
	for _, base := range tf.Bases {
		if base.Name == target.From {
			return base.Image, nil
		}
	}
	return "", fmt.Errorf("target %q: no image found (set 'from' or 'from_image')", target.Name)
}

// ResolveWorkdir returns the workdir for a target.
func (tf *Torresmofile) ResolveWorkdir(target *Target) string {
	if target.FromImage != "" {
		return ""
	}
	for _, base := range tf.Bases {
		if base.Name == target.From {
			return base.Workdir
		}
	}
	return ""
}

// GetTarget returns a target by name.
func (tf *Torresmofile) GetTarget(name string) (*Target, error) {
	for i := range tf.Targets {
		if tf.Targets[i].Name == name {
			return &tf.Targets[i], nil
		}
	}
	return nil, fmt.Errorf("target %q not found", name)
}

// TargetNames returns all target names.
func (tf *Torresmofile) TargetNames() []string {
	names := make([]string, len(tf.Targets))
	for i, tgt := range tf.Targets {
		names[i] = tgt.Name
	}
	return names
}

// GetEnvVars collects all environment variables from a target's env blocks.
func (t *Target) GetEnvVars() map[string]string {
	env := make(map[string]string)
	for _, block := range t.EnvBlocks {
		if block.Remain == nil {
			continue
		}
		attrs, diags := block.Remain.JustAttributes()
		if diags.HasErrors() {
			continue
		}
		for name, attr := range attrs {
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				continue
			}
			if val.Type() == cty.String {
				env[name] = val.AsString()
			}
		}
	}
	return env
}


