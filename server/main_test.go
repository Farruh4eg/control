package main

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestGenerateRandomHostID(t *testing.T) {

	id1 := generateRandomHostID(4)
	expectedLength1 := 8
	if len(id1) != expectedLength1 {
		t.Errorf("TestGenerateRandomHostID: Ожидалась длина %d, получено %d для ID: %s", expectedLength1, len(id1), id1)
	}

	id2 := generateRandomHostID(8)
	expectedLength2 := 16
	if len(id2) != expectedLength2 {
		t.Errorf("TestGenerateRandomHostID: Ожидалась длина %d, получено %d для ID: %s", expectedLength2, len(id2), id2)
	}

	id3 := generateRandomHostID(4)
	if id1 == id3 && !strings.HasPrefix(id1, "randfail") {

		t.Logf("TestGenerateRandomHostID: ID1 (%s) и ID3 (%s) совпали. Это возможно, но маловероятно для случайных ID.", id1, id3)
	}

	if id1 == "" {
		t.Errorf("TestGenerateRandomHostID: Сгенерирован пустой ID.")
	}
}

func TestIsNetworkCloseError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{"NilError", nil, false},
		{"EOFError", io.EOF, true},
		{"ClosedNetworkError", fmt.Errorf("read tcp 127.0.0.1:1234->127.0.0.1:5678: use of closed network connection"), true},
		{"ConnectionResetError", fmt.Errorf("read: connection reset by peer"), true},
		{"BrokenPipeError", fmt.Errorf("write: broken pipe"), true},
		{"ForciblyClosedError", fmt.Errorf("wsarecv: An existing connection was forcibly closed by the remote host."), true},
		{"OtherError", fmt.Errorf("это другая ошибка"), false},
		{"EmptyErrorString", fmt.Errorf(""), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := isNetworkCloseError(tc.err)
			if result != tc.expected {
				t.Errorf("isNetworkCloseError(%v): ожидалось %v, получено %v", tc.err, tc.expected, result)
			}
		})
	}
}
