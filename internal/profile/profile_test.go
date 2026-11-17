package profile

import "testing"

func TestGet(t *testing.T) {
	for _, name := range []string{"light", "standard", "heavy"} {
		p, err := Get(name)
		if err != nil {
			t.Errorf("Get(%q): %v", name, err)
			continue
		}
		if p.Name != name {
			t.Errorf("Get(%q).Name = %q", name, p.Name)
		}
		if p.CPU <= 0 {
			t.Errorf("Get(%q).CPU = %f, want > 0", name, p.CPU)
		}
		if p.Memory <= 0 {
			t.Errorf("Get(%q).Memory = %d, want > 0", name, p.Memory)
		}
		if p.MaxPods <= 0 {
			t.Errorf("Get(%q).MaxPods = %d, want > 0", name, p.MaxPods)
		}
		if p.Description == "" {
			t.Errorf("Get(%q).Description is empty", name)
		}
	}
}

func TestGetUnknown(t *testing.T) {
	_, err := Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown profile, got nil")
	}
}

func TestList(t *testing.T) {
	profiles := List()
	if len(profiles) != 3 {
		t.Fatalf("List() returned %d profiles, want 3", len(profiles))
	}

	names := make(map[string]bool)
	for _, p := range profiles {
		names[p.Name] = true
	}
	for _, want := range []string{"light", "standard", "heavy"} {
		if !names[want] {
			t.Errorf("List() missing profile %q", want)
		}
	}
}

func TestDefault(t *testing.T) {
	if got := Default(); got != "standard" {
		t.Errorf("Default() = %q, want standard", got)
	}
}

func TestProfileValues(t *testing.T) {
	light, _ := Get("light")
	standard, _ := Get("standard")
	heavy, _ := Get("heavy")

	if light.CPU >= standard.CPU {
		t.Errorf("light.CPU (%f) >= standard.CPU (%f)", light.CPU, standard.CPU)
	}
	if standard.CPU >= heavy.CPU {
		t.Errorf("standard.CPU (%f) >= heavy.CPU (%f)", standard.CPU, heavy.CPU)
	}

	if light.Memory >= standard.Memory {
		t.Errorf("light.Memory (%d) >= standard.Memory (%d)", light.Memory, standard.Memory)
	}
	if standard.Memory >= heavy.Memory {
		t.Errorf("standard.Memory (%d) >= heavy.Memory (%d)", standard.Memory, heavy.Memory)
	}

	if light.MaxPods >= standard.MaxPods {
		t.Errorf("light.MaxPods (%d) >= standard.MaxPods (%d)", light.MaxPods, standard.MaxPods)
	}
	if standard.MaxPods >= heavy.MaxPods {
		t.Errorf("standard.MaxPods (%d) >= heavy.MaxPods (%d)", standard.MaxPods, heavy.MaxPods)
	}
}
