package vault

import "testing"

func TestUnlockedVaultSealsAndOpensValues(t *testing.T) {
	Lock()
	t.Cleanup(Lock)
	if _, err := Seal("secret"); err == nil {
		t.Fatal("locked vault sealed a value")
	}
	if err := Unlock("correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	ciphertext, err := Seal("extension secret")
	if err != nil {
		t.Fatal(err)
	}
	if ciphertext == "extension secret" {
		t.Fatal("secret was not encrypted")
	}
	plaintext, err := Open(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if plaintext != "extension secret" {
		t.Fatalf("plaintext=%q", plaintext)
	}
	Lock()
	if _, err := Open(ciphertext); err == nil {
		t.Fatal("locked vault opened a value")
	}
}
