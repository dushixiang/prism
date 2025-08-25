package tools

import "testing"

func TestGenerateRandomID(t *testing.T) {
	for i := 0; i < 10; i++ {
		randomID, err := GenerateRandomID()
		if err != nil {
			t.Fatal(err)
		}
		t.Log(randomID)
	}
}
