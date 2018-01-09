package handlers

import "testing"

func TestExtractHashTags(t *testing.T) {
	res := extractHashTags("hello #vbambuke Проверка #тес123т")
	if len(res) != 2 {
		t.Fatalf("Invalid number of matches: got %d, expected %d", len(res), 2)
	}

	if res[0] != "vbambuke" {
		t.Fatalf("Invalid match #0: got %s, expected %s", res[0], "vbambuke")
	}

	if res[1] != "тес123т" {
		t.Fatalf("Invalid match #1: got %s, expected %s", res[1], "тес123т")
	}
}
