package aip2p

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestDeriveHDKeyMatchesSLIP10Vector2Root(t *testing.T) {
	t.Parallel()

	seed := mustDecodeHex(t, "fffcf9f6f3f0edeae7e4e1dedbd8d5d2cfccc9c6c3c0bdbab7b4b1aeaba8a5a29f9c999693908d8a8784817e7b7875726f6c696663605d5a5754514e4b484542")
	publicKey, privateKey, chainCode, err := DeriveHDKey(seed, "m")
	if err != nil {
		t.Fatalf("DeriveHDKey() error = %v", err)
	}
	if chainCode != "ef70a74db9c3a5af931b5fe73ed8e1a53464133654fd55e7a66f8570b8e33c3b" {
		t.Fatalf("chainCode = %q", chainCode)
	}
	if seedHex(privateKey) != "171cb88b1b3c1db25add599712e36245d75bc65a1a5c9e18d76f9f2b1eab4012" {
		t.Fatalf("private seed = %q", seedHex(privateKey))
	}
	if "00"+publicKey != "008fe9693f8fa62a4305a140b9764c5ee01e455963744fe18204b4fb948249308a" {
		t.Fatalf("publicKey = %q", publicKey)
	}
}

func TestDeriveHDKeyMatchesSLIP10Vector2Child(t *testing.T) {
	t.Parallel()

	seed := mustDecodeHex(t, "fffcf9f6f3f0edeae7e4e1dedbd8d5d2cfccc9c6c3c0bdbab7b4b1aeaba8a5a29f9c999693908d8a8784817e7b7875726f6c696663605d5a5754514e4b484542")
	publicKey, privateKey, chainCode, err := DeriveHDKey(seed, "m/0'")
	if err != nil {
		t.Fatalf("DeriveHDKey() error = %v", err)
	}
	if chainCode != "0b78a3226f915c082bf118f83618a618ab6dec793752624cbeb622acb562862d" {
		t.Fatalf("chainCode = %q", chainCode)
	}
	if seedHex(privateKey) != "1559eb2bbec5790b0c65d8693e4d0875b1747f4970ae8b650486ed7470845635" {
		t.Fatalf("private seed = %q", seedHex(privateKey))
	}
	if "00"+publicKey != "0086fab68dcb57aa196c77c5f264f215a112c22a912c10d123b0d03c3c28ef1037" {
		t.Fatalf("publicKey = %q", publicKey)
	}
}

func TestPathFromURIUsesDeterministicSegments(t *testing.T) {
	t.Parallel()

	pathOne, err := PathFromURI("agent://alice/work")
	if err != nil {
		t.Fatalf("PathFromURI() error = %v", err)
	}
	pathTwo, err := PathFromURI("agent://alice/work")
	if err != nil {
		t.Fatalf("PathFromURI() error = %v", err)
	}
	pathThree, err := PathFromURI("agent://alice/personal")
	if err != nil {
		t.Fatalf("PathFromURI() error = %v", err)
	}
	if pathOne != pathTwo {
		t.Fatalf("work paths differ: %q vs %q", pathOne, pathTwo)
	}
	if pathOne == pathThree {
		t.Fatalf("distinct segments should not map to the same path: %q", pathOne)
	}
	if !strings.HasPrefix(pathOne, "m/0'/") {
		t.Fatalf("path = %q", pathOne)
	}
}

func TestParseDerivationPathRejectsNonHardenedEd25519Segment(t *testing.T) {
	t.Parallel()

	if _, err := ParseDerivationPath("m/0"); err == nil {
		t.Fatal("expected non-hardened path error")
	}
}

func TestMnemonicRoundTrip(t *testing.T) {
	t.Parallel()

	mnemonic, err := GenerateMnemonic()
	if err != nil {
		t.Fatalf("GenerateMnemonic() error = %v", err)
	}
	if len(strings.Fields(mnemonic)) != 24 {
		t.Fatalf("word count = %d", len(strings.Fields(mnemonic)))
	}
	seed, err := MnemonicToSeed(mnemonic)
	if err != nil {
		t.Fatalf("MnemonicToSeed() error = %v", err)
	}
	if len(seed) != 64 {
		t.Fatalf("seed len = %d", len(seed))
	}
}

func seedHex(privateKey string) string {
	if len(privateKey) < 64 {
		return privateKey
	}
	return privateKey[:64]
}

func mustDecodeHex(t *testing.T, value string) []byte {
	t.Helper()
	out, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("DecodeString(%q) error = %v", value, err)
	}
	return out
}
