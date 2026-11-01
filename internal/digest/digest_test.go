package digest

import "testing"

// The digest string is stored in hub metadata, receipts and released digest
// lists, so its value and form must never change.
func TestBytesKeepsTheStoredDigestForm(t *testing.T) {
	t.Parallel()
	const want = "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := Bytes([]byte("abc")); got != want {
		t.Fatalf("Bytes(abc) = %s, want %s", got, want)
	}
	if !Valid(want) {
		t.Fatalf("Valid rejects the form Bytes returns")
	}
}

func TestValidAcceptsOnlyLowercaseSHA256Hex(t *testing.T) {
	t.Parallel()
	hex64 := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	for _, value := range []string{
		"",
		hex64,
		"sha256:" + hex64[:63],
		"sha256:" + hex64 + "0",
		"sha256:" + hex64 + "\n",
		"sha256:BA7816BF8F01CFEA414140DE5DAE2223B00361A396177A9CB410FF61F20015AD",
		"sha256:" + hex64[:63] + "g",
		"sha512:" + hex64,
	} {
		if Valid(value) {
			t.Errorf("Valid(%q) = true, want false", value)
		}
	}
}
