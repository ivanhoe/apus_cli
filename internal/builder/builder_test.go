package builder

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStripApusPins_V2Format(t *testing.T) {
	input := `{
  "pins" : [
    {
      "identity" : "apus",
      "kind" : "remoteSourceControl",
      "location" : "https://github.com/ivanhoe/apus",
      "state" : {
        "branch" : "main",
        "revision" : "abc"
      }
    },
    {
      "identity" : "nuke",
      "kind" : "remoteSourceControl",
      "location" : "https://github.com/kean/Nuke",
      "state" : {
        "revision" : "def",
        "version" : "12.0.0"
      }
    }
  ],
  "version" : 2
}`

	updated, changed, err := stripApusPins([]byte(input))
	if err != nil {
		t.Fatalf("stripApusPins() error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}
	if strings.Contains(string(updated), "github.com/ivanhoe/apus") {
		t.Fatalf("expected apus pin to be removed")
	}
	if !strings.Contains(string(updated), "github.com/kean/Nuke") {
		t.Fatalf("expected other pins to remain")
	}
}

func TestStripApusPins_V1Format(t *testing.T) {
	input := `{
  "object" : {
    "pins" : [
      {
        "package" : "Apus",
        "repositoryURL" : "https://github.com/ivanhoe/apus",
        "state" : {
          "version" : "0.3.0"
        }
      },
      {
        "package" : "Nuke",
        "repositoryURL" : "https://github.com/kean/Nuke",
        "state" : {
          "version" : "12.0.0"
        }
      }
    ]
  },
  "version" : 1
}`

	updated, changed, err := stripApusPins([]byte(input))
	if err != nil {
		t.Fatalf("stripApusPins() error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(updated, &parsed); err != nil {
		t.Fatalf("invalid updated json: %v", err)
	}

	if strings.Contains(string(updated), "github.com/ivanhoe/apus") {
		t.Fatalf("expected apus pin to be removed")
	}
	if !strings.Contains(string(updated), "github.com/kean/Nuke") {
		t.Fatalf("expected other pins to remain")
	}
}

func TestStripApusPins_NoChange(t *testing.T) {
	input := `{
  "pins" : [
    {
      "identity" : "nuke",
      "kind" : "remoteSourceControl",
      "location" : "https://github.com/kean/Nuke",
      "state" : {
        "revision" : "def"
      }
    }
  ],
  "version" : 2
}`

	updated, changed, err := stripApusPins([]byte(input))
	if err != nil {
		t.Fatalf("stripApusPins() error: %v", err)
	}
	if changed {
		t.Fatalf("expected changed=false")
	}
	if string(updated) != input {
		t.Fatalf("expected output to match input when unchanged")
	}
}

func TestLooksLikeApusResolutionError(t *testing.T) {
	out := "Could not resolve package dependencies:\n* 'apus' from https://github.com/ivanhoe/apus"
	if !looksLikeApusResolutionError(out) {
		t.Fatalf("expected apus resolution error to be detected")
	}

	if looksLikeApusResolutionError("random failure") {
		t.Fatalf("unexpected apus resolution detection")
	}
}
