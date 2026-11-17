package pipeline

import (
	"sort"
	"strings"
	"testing"
)

func makeTorresmofile(targets []Target) *Torresmofile {
	return &Torresmofile{
		Version: "1",
		Bases:   []Base{{Name: "base", Image: "alpine:3.19"}},
		Targets: targets,
	}
}

func target(name string, deps ...string) Target {
	return Target{
		Name:      name,
		FromImage: "alpine:3.19",
		DependsOn: deps,
	}
}

func TestBuildDAGSimple(t *testing.T) {
	tf := makeTorresmofile([]Target{
		target("a"),
		target("b"),
		target("c"),
	})

	dag, err := BuildDAG(tf, "")
	if err != nil {
		t.Fatalf("BuildDAG: %v", err)
	}

	levels := dag.Levels()
	if len(levels) != 1 {
		t.Fatalf("levels = %d, want 1", len(levels))
	}
	sort.Strings(levels[0])
	want := []string{"a", "b", "c"}
	if len(levels[0]) != 3 {
		t.Fatalf("level[0] = %v, want %v", levels[0], want)
	}
	for i, name := range want {
		if levels[0][i] != name {
			t.Errorf("level[0][%d] = %q, want %q", i, levels[0][i], name)
		}
	}
}

func TestBuildDAGLinear(t *testing.T) {
	tf := makeTorresmofile([]Target{
		target("a"),
		target("b", "a"),
		target("c", "b"),
	})

	dag, err := BuildDAG(tf, "")
	if err != nil {
		t.Fatalf("BuildDAG: %v", err)
	}

	levels := dag.Levels()
	if len(levels) != 3 {
		t.Fatalf("levels = %d, want 3", len(levels))
	}
	if levels[0][0] != "a" {
		t.Errorf("level[0] = %v, want [a]", levels[0])
	}
	if levels[1][0] != "b" {
		t.Errorf("level[1] = %v, want [b]", levels[1])
	}
	if levels[2][0] != "c" {
		t.Errorf("level[2] = %v, want [c]", levels[2])
	}
}

func TestBuildDAGDiamond(t *testing.T) {
	tf := makeTorresmofile([]Target{
		target("a"),
		target("b", "a"),
		target("c", "a"),
		target("d", "b", "c"),
	})

	dag, err := BuildDAG(tf, "")
	if err != nil {
		t.Fatalf("BuildDAG: %v", err)
	}

	levels := dag.Levels()
	if len(levels) != 3 {
		t.Fatalf("levels = %d, want 3", len(levels))
	}
	if levels[0][0] != "a" {
		t.Errorf("level[0] = %v, want [a]", levels[0])
	}

	sort.Strings(levels[1])
	if len(levels[1]) != 2 || levels[1][0] != "b" || levels[1][1] != "c" {
		t.Errorf("level[1] = %v, want [b c]", levels[1])
	}
	if levels[2][0] != "d" {
		t.Errorf("level[2] = %v, want [d]", levels[2])
	}
}

func TestBuildDAGSelectedTarget(t *testing.T) {
	tf := makeTorresmofile([]Target{
		target("a"),
		target("b", "a"),
		target("c", "b"),
		target("unrelated"),
	})

	dag, err := BuildDAG(tf, "c")
	if err != nil {
		t.Fatalf("BuildDAG: %v", err)
	}

	order := dag.Order()
	if len(order) != 3 {
		t.Fatalf("order = %v, want 3 targets (a, b, c)", order)
	}

	included := map[string]bool{}
	for _, name := range order {
		included[name] = true
	}
	if !included["a"] || !included["b"] || !included["c"] {
		t.Errorf("order = %v, want a, b, c included", order)
	}
	if included["unrelated"] {
		t.Error("unrelated target should not be included")
	}

	levels := dag.Levels()
	if len(levels) != 3 {
		t.Fatalf("levels = %d, want 3", len(levels))
	}
}

func TestBuildDAGCycle(t *testing.T) {
	tf := &Torresmofile{
		Version: "1",
		Targets: []Target{
			{Name: "a", FromImage: "alpine:3.19", DependsOn: []string{"b"}},
			{Name: "b", FromImage: "alpine:3.19", DependsOn: []string{"a"}},
		},
	}

	_, err := BuildDAG(tf, "")
	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("error = %q, want 'cycle' substring", err)
	}
}

func TestBuildDAGUndefinedDep(t *testing.T) {
	tf := &Torresmofile{
		Version: "1",
		Targets: []Target{
			{Name: "a", FromImage: "alpine:3.19", DependsOn: []string{"missing"}},
		},
	}

	_, err := BuildDAG(tf, "")
	if err == nil {
		t.Fatal("expected error for undefined dependency, got nil")
	}
	if !strings.Contains(err.Error(), "unknown target") {
		t.Errorf("error = %q, want 'unknown target' substring", err)
	}
}
