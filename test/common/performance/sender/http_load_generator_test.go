package sender

import (
	"testing"
)

func TestGenerateRandStringPayload(t *testing.T) {
	const sizeRandomPayload = 10

	generated := generateRandStringPayload(sizeRandomPayload)

	if len(generated) != sizeRandomPayload {
		t.Errorf("len(generateRandStringPayload(sizeRandomPayload)) = %v, want %v", len(generated), sizeRandomPayload)
	}

	if generated[0] != markLetter {
		t.Errorf("generateRandStringPayload(sizeRandomPayload)[0] = %v, want %v", generated[0], markLetter)
	}

	if generated[sizeRandomPayload-1] != markLetter {
		t.Errorf("generateRandStringPayload(sizeRandomPayload)[sizeRandomPayload - 1] = %v, want %v", generated[sizeRandomPayload-1], markLetter)
	}
}
