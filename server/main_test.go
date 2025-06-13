package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	pb "control_grpc/gen/proto"
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

func TestKeyboardLogging(t *testing.T) {
	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer)
	originalFlags := log.Flags()
	log.SetFlags(log.Lshortfile)
	defer func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(originalFlags)
	}()

	reqKeyDown := &pb.FeedRequest{
		Message:           "keyboard_event",
		KeyboardEventType: "keydown",
		KeyName:           "KeyB",
		ModifierCtrl:      true,
		Timestamp:         time.Now().UnixNano(),
	}
	processKeyboardInput(reqKeyDown)
	logOutput := logBuffer.String()

	if !strings.Contains(logOutput, "remote_control_service.go:") {
		t.Errorf("TestKeyboardLogging KeyDown: Log output does not contain source file info: %s", logOutput)
	}
	if !strings.Contains(logOutput, "Received KeyboardEvent: Type='keydown', FyneKeyName='KeyB'") {
		t.Errorf("TestKeyboardLogging KeyDown: Log output does not contain correct receive message: %s", logOutput)
	}
	if !strings.Contains(logOutput, "Modifiers: Shift[false], Ctrl[true]") {
		t.Errorf("TestKeyboardLogging KeyDown: Log output does not contain correct modifiers: %s", logOutput)
	}
	if !strings.Contains(logOutput, "Mapped FyneKeyName 'KeyB' to robotgoKeyName 'b'") {
		t.Errorf("TestKeyboardLogging KeyDown: Log output does not contain correct mapping: %s", logOutput)
	}
	if !strings.Contains(logOutput, "Action: Tapping key 'b'") {
		t.Errorf("TestKeyboardLogging KeyDown: Log output does not contain correct action: %s", logOutput)
	}
	logBuffer.Reset()

	reqKeyChar := &pb.FeedRequest{
		Message:           "keyboard_event",
		KeyboardEventType: "keychar",
		KeyCharStr:        "@",
		Timestamp:         time.Now().UnixNano(),
	}
	processKeyboardInput(reqKeyChar)
	logOutputChar := logBuffer.String()

	if !strings.Contains(logOutputChar, "remote_control_service.go:") {
		t.Errorf("TestKeyboardLogging KeyChar: Log output does not contain source file info: %s", logOutputChar)
	}
	if !strings.Contains(logOutputChar, "Received KeyboardEvent: Type='keychar', FyneKeyName='', KeyChar='@'") {
		t.Errorf("TestKeyboardLogging KeyChar: Log output does not contain correct receive message: %s", logOutputChar)
	}
	if !strings.Contains(logOutputChar, "Action: Typing character from keychar event '@'") {
		t.Errorf("TestKeyboardLogging KeyChar: Log output does not contain correct action: %s", logOutputChar)
	}
	logBuffer.Reset()

	reqModDown := &pb.FeedRequest{
		Message:           "keyboard_event",
		KeyboardEventType: "keydown",
		KeyName:           "ShiftL",
		Timestamp:         time.Now().UnixNano(),
	}
	processKeyboardInput(reqModDown)
	logOutputModDown := logBuffer.String()

	if !strings.Contains(logOutputModDown, "remote_control_service.go:") {
		t.Errorf("TestKeyboardLogging ModKeyDown: Log output does not contain source file info: %s", logOutputModDown)
	}
	if !strings.Contains(logOutputModDown, "Received KeyboardEvent: Type='keydown', FyneKeyName='ShiftL'") {
		t.Errorf("TestKeyboardLogging ModKeyDown: Log output does not contain correct receive message: %s", logOutputModDown)
	}
	if !strings.Contains(logOutputModDown, "Mapped FyneKeyName 'ShiftL' to robotgoKeyName 'shift'") {
		t.Errorf("TestKeyboardLogging ModKeyDown: Log output does not contain correct mapping: %s", logOutputModDown)
	}
	if !strings.Contains(logOutputModDown, "Action: Modifier 'shift' pressed down") {
		t.Errorf("TestKeyboardLogging ModKeyDown: Log output does not contain correct action: %s", logOutputModDown)
	}
	logBuffer.Reset()

	reqModUp := &pb.FeedRequest{
		Message:           "keyboard_event",
		KeyboardEventType: "keyup",
		KeyName:           "ShiftL",
		Timestamp:         time.Now().UnixNano(),
	}
	processKeyboardInput(reqModUp)
	logOutputModUp := logBuffer.String()

	if !strings.Contains(logOutputModUp, "remote_control_service.go:") {
		t.Errorf("TestKeyboardLogging ModKeyUp: Log output does not contain source file info: %s", logOutputModUp)
	}
	if !strings.Contains(logOutputModUp, "Received KeyboardEvent: Type='keyup', FyneKeyName='ShiftL'") {
		t.Errorf("TestKeyboardLogging ModKeyUp: Log output does not contain correct receive message: %s", logOutputModUp)
	}
	if !strings.Contains(logOutputModUp, "Mapped FyneKeyName 'ShiftL' to robotgoKeyName 'shift'") {
		t.Errorf("TestKeyboardLogging ModKeyUp: Log output does not contain correct mapping: %s", logOutputModUp)
	}
	if !strings.Contains(logOutputModUp, "Action: Modifier 'shift' released") {
		t.Errorf("TestKeyboardLogging ModKeyUp: Log output does not contain correct action: %s", logOutputModUp)
	}
	logBuffer.Reset()
}
