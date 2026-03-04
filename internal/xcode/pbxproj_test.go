package xcode

import (
	"strings"
	"testing"
)

func TestMigrateLegacyApusRequirement(t *testing.T) {
	input := `repositoryURL = "https://github.com/ivanhoe/apus";
			requirement = {
				kind = upToNextMajorVersion;
				minimumVersion = 0.3.0;
			};`

	got := migrateLegacyApusRequirement(input)

	if strings.Contains(got, "upToNextMajorVersion") {
		t.Fatalf("legacy requirement should be removed, got:\n%s", got)
	}
	if strings.Contains(got, "minimumVersion = 0.3.0;") {
		t.Fatalf("legacy minimum version should be removed, got:\n%s", got)
	}
	if !strings.Contains(got, "kind = branch;") {
		t.Fatalf("expected branch requirement, got:\n%s", got)
	}
	if !strings.Contains(got, "branch = main;") {
		t.Fatalf("expected branch main requirement, got:\n%s", got)
	}
}

func TestMigrateLegacyApusRequirement_NoLegacy(t *testing.T) {
	input := `repositoryURL = "https://github.com/ivanhoe/apus";
			requirement = {
				kind = branch;
				branch = main;
			};`

	got := migrateLegacyApusRequirement(input)
	if got != input {
		t.Fatalf("expected no-op for non-legacy requirement")
	}
}

func TestMigrateLegacyApusRequirement_VariedFormatting(t *testing.T) {
	input := `repositoryURL = "https://github.com/ivanhoe/apus";
requirement = {
    kind = upToNextMajorVersion;
    minimumVersion = "0.3.0";
};`

	got := migrateLegacyApusRequirement(input)

	if strings.Contains(got, "upToNextMajorVersion") {
		t.Fatalf("legacy requirement should be removed, got:\n%s", got)
	}
	if strings.Contains(got, "minimumVersion") {
		t.Fatalf("legacy minimumVersion should be removed, got:\n%s", got)
	}
	if !strings.Contains(got, "kind = branch;") || !strings.Contains(got, "branch = main;") {
		t.Fatalf("expected branch main requirement, got:\n%s", got)
	}
}

func TestNormalizeLocalApusReference(t *testing.T) {
	input := `
		ABCDEFABCDEFABCDEFABCDEF /* Apus */ = {
			isa = XCSwiftPackageProductDependency;
			package = AAAAAAAAAAAAAAAAAAAAAAA1 /* XCLocalSwiftPackageReference "../apus" */;
			productName = Apus;
		};
		EEEEEEEEEEEEEEEEEEEEEEEE /* XCRemoteSwiftPackageReference "Apus" */ = {
			isa = XCRemoteSwiftPackageReference;
			repositoryURL = "https://github.com/ivanhoe/apus";
			requirement = {
				kind = branch;
				branch = main;
			};
		};
		packageReferences = (
			AAAAAAAAAAAAAAAAAAAAAAA1 /* XCLocalSwiftPackageReference "../apus" */,
		);
		AAAAAAAAAAAAAAAAAAAAAAA1 /* XCLocalSwiftPackageReference "../apus" */ = {
			isa = XCLocalSwiftPackageReference;
			relativePath = ../apus;
		};
`

	got, err := normalizeLocalApusReference(input)
	if err != nil {
		t.Fatalf("normalizeLocalApusReference() error: %v", err)
	}

	if strings.Contains(got, `XCLocalSwiftPackageReference "../apus"`) {
		t.Fatalf("local Apus package reference should be removed:\n%s", got)
	}
	if !strings.Contains(got, `package = EEEEEEEEEEEEEEEEEEEEEEEE /* XCRemoteSwiftPackageReference "Apus" */;`) {
		t.Fatalf("Apus dependency should point to remote package reference:\n%s", got)
	}
}

func TestNormalizeLocalApusReference_NoRemote(t *testing.T) {
	input := `
		ABCDEFABCDEFABCDEFABCDEF /* Apus */ = {
			isa = XCSwiftPackageProductDependency;
			package = AAAAAAAAAAAAAAAAAAAAAAA1 /* XCLocalSwiftPackageReference "../apus" */;
			productName = Apus;
		};
		AAAAAAAAAAAAAAAAAAAAAAA1 /* XCLocalSwiftPackageReference "../apus" */ = {
			isa = XCLocalSwiftPackageReference;
			relativePath = ../apus;
		};
`

	_, err := normalizeLocalApusReference(input)
	if err == nil {
		t.Fatalf("expected error when no Apus remote reference exists")
	}
}
