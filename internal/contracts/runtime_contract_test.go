package contracts

import "testing"

func TestBuildContractManifestMatchesSpecV1(t *testing.T) {
	t.Parallel()

	manifest := BuildContractManifest()

	if manifest.Version != "v1" {
		t.Fatalf("expected contract version v1, got %q", manifest.Version)
	}

	if len(manifest.BoundMethods) != 32 {
		t.Fatalf("expected 32 bound methods, got %d", len(manifest.BoundMethods))
	}
	for _, expected := range []BoundMethod{
		MethodGitHubWorkspaceLink,
		MethodGitHubWorkspaceStatus,
		MethodGitHubWorkspacePull,
		MethodGitHubWorkspacePush,
		MethodGitHubWorkspaceUnlink,
	} {
		found := false
		for _, method := range manifest.BoundMethods {
			if method == string(expected) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected bound method %q", expected)
		}
	}

	if len(manifest.EventNames) != 5 {
		t.Fatalf("expected 5 event names, got %d", len(manifest.EventNames))
	}

	if len(manifest.StreamStates) != 8 {
		t.Fatalf("expected 8 canonical stream states, got %d", len(manifest.StreamStates))
	}

	if len(manifest.Modules) != 7 {
		t.Fatalf("expected 7 component modules, got %d", len(manifest.Modules))
	}
}

func TestCanTransitionFollowsCanonicalStreamRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		from StreamState
		to   StreamState
		want bool
	}{
		{name: "idle to connecting", from: StreamStateIdle, to: StreamStateConnecting, want: true},
		{name: "connecting to open", from: StreamStateConnecting, to: StreamStateOpen, want: true},
		{name: "open to half_closed_local", from: StreamStateOpen, to: StreamStateHalfClosedLocal, want: true},
		{name: "open to error", from: StreamStateOpen, to: StreamStateError, want: true},
		{name: "closed to open disallowed", from: StreamStateClosed, to: StreamStateOpen, want: false},
		{name: "cancelled to connecting disallowed", from: StreamStateCancelled, to: StreamStateConnecting, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := CanTransition(tt.from, tt.to)
			if got != tt.want {
				t.Fatalf("CanTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
