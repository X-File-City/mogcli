package profile

import (
	"testing"

	"github.com/jaredpalmer/mogcli/internal/config"
)

func TestNormalizeName(t *testing.T) {
	if _, err := NormalizeName("   "); err == nil {
		t.Fatal("expected error for empty profile name")
	}
	name, err := NormalizeName(" work ")
	if err != nil {
		t.Fatalf("NormalizeName failed: %v", err)
	}
	if name != "work" {
		t.Fatalf("unexpected normalized name: %q", name)
	}
}

func TestValidateRecord(t *testing.T) {
	record := config.ProfileRecord{
		Name:     "work",
		Audience: AudienceEnterprise,
		ClientID: "abc",
		AuthMode: AuthModeDelegated,
	}
	if err := ValidateRecord(record); err != nil {
		t.Fatalf("ValidateRecord failed: %v", err)
	}

	record.Audience = "invalid"
	if err := ValidateRecord(record); err == nil {
		t.Fatal("expected invalid audience error")
	}
}
