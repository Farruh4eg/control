package main

import (
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"log"
	"sync"
	"time"

	pb "control_grpc/gen/proto"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type mouseOverlay struct {
	widget.BaseWidget
	inputEventsChan chan<- *pb.FeedRequest
	mouseBtnState   string
	mu              sync.Mutex
	window          fyne.Window
	isShiftDown     bool
	isCtrlDown      bool
	isAltDown       bool
	isSuperDown     bool

	batchedMoves []*pb.MouseMovePoint
	batchTicker  *time.Ticker
	batchMutex   sync.Mutex
	lastMoveTime time.Time
}

func newMouseOverlay(inputChan chan<- *pb.FeedRequest, win fyne.Window) *mouseOverlay {
	mo := &mouseOverlay{
		inputEventsChan: inputChan,
		window:          win,
		isShiftDown:     false,
		isCtrlDown:      false,
		isAltDown:       false,
		isSuperDown:     false,
		batchedMoves:    make([]*pb.MouseMovePoint, 0),
		batchTicker:     time.NewTicker(20 * time.Millisecond),
	}
	mo.ExtendBaseWidget(mo)

	go func() {
		defer func() {

			mo.batchTicker.Stop()
			log.Println("Mouse batching ticker goroutine stopped.")
		}()

		for range mo.batchTicker.C {
			mo.sendBatchedMoves()
		}
	}()

	return mo
}

func (mo *mouseOverlay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewWithoutLayout())
}

func (mo *mouseOverlay) Focusable() bool {
	return true
}

func (mo *mouseOverlay) FocusGained() {

}

func (mo *mouseOverlay) FocusLost() {

}

func (mo *mouseOverlay) TypedKey(ev *fyne.KeyEvent) {
	if !canControlKeyboard {
		log.Println("TypedKey event dropped: Keyboard control denied by host permissions.")
		return
	}
	var pbReq *pb.FeedRequest = nil
	keyboardEventType := "keydown"

	switch ev.Name {
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		mo.isShiftDown = !mo.isShiftDown
		if !mo.isShiftDown {
			keyboardEventType = "keyup"
		}
		log.Printf("Modifier Key: Shift, New State: %s", keyboardEventType)
		pbReq = &pb.FeedRequest{
			Message:           "keyboard_event",
			KeyboardEventType: keyboardEventType,
			KeyName:           "shift",
		}
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		mo.isCtrlDown = !mo.isCtrlDown
		if !mo.isCtrlDown {
			keyboardEventType = "keyup"
		}
		log.Printf("Modifier Key: Ctrl, New State: %s", keyboardEventType)
		pbReq = &pb.FeedRequest{
			Message:           "keyboard_event",
			KeyboardEventType: keyboardEventType,
			KeyName:           "ctrl",
		}
	case desktop.KeyAltLeft, desktop.KeyAltRight, desktop.KeyMenu:
		mo.isAltDown = !mo.isAltDown
		if !mo.isAltDown {
			keyboardEventType = "keyup"
		}
		log.Printf("Modifier Key: Alt, New State: %s", keyboardEventType)
		pbReq = &pb.FeedRequest{
			Message:           "keyboard_event",
			KeyboardEventType: keyboardEventType,
			KeyName:           "alt",
		}
	case desktop.KeySuperLeft, desktop.KeySuperRight:
		mo.isSuperDown = !mo.isSuperDown
		if !mo.isSuperDown {
			keyboardEventType = "keyup"
		}
		log.Printf("Modifier Key: Super, New State: %s", keyboardEventType)
		pbReq = &pb.FeedRequest{
			Message:           "keyboard_event",
			KeyboardEventType: keyboardEventType,
			KeyName:           "super",
		}
	default:
		keyNameStr := string(ev.Name)

		// Check for special keys that also produce characters via TypedRune
		switch keyNameStr {
		case "Space", "Return", "Tab":
			log.Printf("TypedKey: Key '%s' received. Physical: %s. Ignoring this TypedKey event as TypedRune will handle its character output.", keyNameStr, ev.Physical)
			// Do nothing, pbReq remains nil
		default:
			// Existing logic for other keys
			if keyNameStr == "" {
				log.Printf("TypedKey: Empty ev.Name received. Physical: %s. Likely handled by TypedRune. Ignoring this TypedKey event.", ev.Physical)
			} else if len(keyNameStr) == 1 {
				// If keyNameStr is a single character, it's assumed to be a printable character (including Unicode)
				// that will be handled by TypedRune. Log this and do not create a pbReq for TypedKey.
				// This handles cases like English letters, Russian letters, numbers, and symbols.
				log.Printf("TypedKey: Single character key '%s' received. Physical: %s. Ignoring this TypedKey event as TypedRune will handle it.", keyNameStr, ev.Physical)
			} else {
				// This block now handles non-character-producing special keys like "BackSpace", "ArrowLeft", "Shift", etc.
				// (Modifier keys like Shift, Ctrl, Alt, Super are handled earlier in the TypedKey function).
				log.Printf("TypedKey: Special Key (non-character or modifier): '%s', Physical: %s", keyNameStr, ev.Physical)
				pbReq = &pb.FeedRequest{
					Message:           "keyboard_event",
					KeyboardEventType: "keydown",
					KeyName:           keyNameStr,
				}
			}
		}
	}

	if pbReq != nil {
	KeyboardEventType: "keydown",
		KeyName:           keyNameStr,
	}
}
}

if pbReq != nil {
pbReq.ModifierShift = mo.isShiftDown
pbReq.ModifierCtrl = mo.isCtrlDown
pbReq.ModifierAlt = mo.isAltDown
pbReq.ModifierSuper = mo.isSuperDown
pbReq.Timestamp = time.Now().UnixNano()

log.Printf("Client Sending Keyboard Event: Type='%s', KeyName='%s', KeyChar='%s', Shift[%t], Ctrl[%t], Alt[%t], Super[%t]",
pbReq.KeyboardEventType, pbReq.KeyName, pbReq.KeyCharStr,
pbReq.ModifierShift, pbReq.ModifierCtrl, pbReq.ModifierAlt, pbReq.ModifierSuper)

mo.sendBatchedMoves()

select {
case mo.inputEventsChan <- pbReq:
default:
log.Println("Keyboard event (TypedKey) dropped (inputEventsChan channel full)")
}
}
}

func (mo *mouseOverlay) TypedRune(r rune) {
	if !canControlKeyboard {
		log.Println("TypedRune event dropped: Keyboard control denied by host permissions.")
		return
	}
	mo.sendBatchedMoves()
	log.Printf("TypedRune: %c", r)
	req := &pb.FeedRequest{
		Message:           "keyboard_event",
		KeyboardEventType: "keychar",
		KeyCharStr:        string(r),
		Timestamp:         time.Now().UnixNano(),
	}

	select {
	case mo.inputEventsChan <- req:
	default:
		log.Println("Rune event dropped (inputEventsChan channel full)")
	}
}

func (mo *mouseOverlay) TypedShortcut(sc fyne.Shortcut) {

}

func (mo *mouseOverlay) requestFocus() {
	if mo.window != nil && mo.window.Canvas() != nil {

		mo.window.Canvas().Focus(mo)
	}
}

func (mo *mouseOverlay) Tapped(_ *fyne.PointEvent) {

	mo.requestFocus()

}

func (mo *mouseOverlay) scaleCoordinates(pos fyne.Position) (float32, float32) {
	sz := mo.Size()
	if sz.Width == 0 || sz.Height == 0 {
		return 0, 0
	}
	targetWidth := float32(1920.0)
	targetHeight := float32(1080.0)
	scaleX := targetWidth / sz.Width
	scaleY := targetHeight / sz.Height
	return pos.X * scaleX, pos.Y * scaleY
}

func (mo *mouseOverlay) sendMouseEvent(eventType, btn string, pos fyne.Position) {
	if !canControlMouse {
		log.Printf("Mouse event type '%s' (button: '%s') dropped due to host permissions.", eventType, btn)
		return
	}
	sx, sy := mo.scaleCoordinates(pos)
	req := &pb.FeedRequest{
		Message:        "mouse_event",
		MouseX:         int32(sx),
		MouseY:         int32(sy),
		MouseBtn:       btn,
		MouseEventType: eventType,
		ClientWidth:    1920,
		ClientHeight:   1080,
		Timestamp:      time.Now().UnixNano(),
	}

	select {
	case mo.inputEventsChan <- req:

	default:
		log.Println("Mouse event dropped (inputEventsChan channel full)")
	}
}

func (mo *mouseOverlay) MouseIn(_ *desktop.MouseEvent) {

	mo.requestFocus()
	mo.mu.Lock()
	currentBtn := mo.mouseBtnState
	mo.mu.Unlock()
	mo.sendBatchedMoves()
	mo.sendMouseEvent("in", currentBtn, fyne.Position{})
}

func (mo *mouseOverlay) MouseMoved(ev *desktop.MouseEvent) {

	if !canControlMouse {

		return
	}

	sx, sy := mo.scaleCoordinates(ev.Position)

	mo.batchMutex.Lock()

	mo.batchedMoves = append(mo.batchedMoves, &pb.MouseMovePoint{X: int32(sx), Y: int32(sy)})
	mo.lastMoveTime = time.Now()

	mo.batchMutex.Unlock()

}

func (mo *mouseOverlay) sendBatchedMoves() {
	mo.batchMutex.Lock()
	defer mo.batchMutex.Unlock()
	mo.sendBatchedMovesLocked()
}

func (mo *mouseOverlay) sendBatchedMovesLocked() {
	if len(mo.batchedMoves) == 0 {
		return
	}

	movesToSend := make([]*pb.MouseMovePoint, len(mo.batchedMoves))
	copy(movesToSend, mo.batchedMoves)

	req := &pb.FeedRequest{
		Message:           "mouse_event",
		MouseEventType:    "batched_mouse_moves",
		BatchedMouseMoves: movesToSend,
		Timestamp:         time.Now().UnixNano(),
		ClientWidth:       1920,
		ClientHeight:      1080,
	}

	log.Printf("Sending batched mouse moves: %d points", len(req.BatchedMouseMoves))

	select {
	case mo.inputEventsChan <- req:

	default:
		log.Printf("Batched mouse event dropped (inputEventsChan channel full), %d points lost", len(req.BatchedMouseMoves))
	}

	mo.batchedMoves = nil

}

func (mo *mouseOverlay) MouseOut() {

	mo.mu.Lock()
	currentBtn := mo.mouseBtnState
	mo.mu.Unlock()
	mo.sendBatchedMoves()
	mo.sendMouseEvent("out", currentBtn, fyne.Position{})
}

func (mo *mouseOverlay) MouseDown(ev *desktop.MouseEvent) {
	mo.requestFocus()
	mo.sendBatchedMoves()

	var btnStr string
	switch ev.Button {
	case desktop.MouseButtonPrimary:
		btnStr = "left"
	case desktop.MouseButtonSecondary:
		btnStr = "right"
	case desktop.MouseButtonTertiary:
		btnStr = "middle"
	default:
		btnStr = "unknown"
	}
	mo.mu.Lock()
	mo.mouseBtnState = btnStr
	mo.mu.Unlock()
	mo.sendMouseEvent("down", btnStr, ev.Position)
}

func (mo *mouseOverlay) MouseUp(ev *desktop.MouseEvent) {
	mo.sendBatchedMoves()
	var btnStr string
	switch ev.Button {
	case desktop.MouseButtonPrimary:
		btnStr = "left"
	case desktop.MouseButtonSecondary:
		btnStr = "right"
	case desktop.MouseButtonTertiary:
		btnStr = "middle"
	default:
		btnStr = "unknown"
	}
	mo.sendMouseEvent("up", btnStr, ev.Position)
	mo.mu.Lock()
	if mo.mouseBtnState == btnStr {
		mo.mouseBtnState = ""
	}
	mo.mu.Unlock()
}

func (mo *mouseOverlay) sendScrollEvent(scrollX, scrollY float32) {
	if !canControlMouse {
		log.Printf("Scroll event (dX: %.2f, dY: %.2f) dropped due to host permissions.", scrollX, scrollY)
		return
	}
	req := &pb.FeedRequest{
		Message:        "mouse_event",
		MouseEventType: "scroll",
		ScrollX:        scrollX,
		ScrollY:        scrollY,
		ClientWidth:    1920,
		ClientHeight:   1080,
		Timestamp:      time.Now().UnixNano(),
	}

	select {
	case mo.inputEventsChan <- req:

	default:
		log.Println("Scroll event dropped (inputEventsChan channel full)")
	}
}

func (mo *mouseOverlay) Scrolled(ev *fyne.ScrollEvent) {
	mo.requestFocus()
	mo.sendBatchedMoves()
	mo.sendScrollEvent(ev.Scrolled.DX, ev.Scrolled.DY)
}

func forwardVideoFeed(stream pb.RemoteControlService_GetFeedClient, ffmpegInput io.Writer) {
	defer func() {
		log.Println("ForwardVideoFeed: Goroutine stopped.")
		if closer, ok := ffmpegInput.(io.Closer); ok {
			log.Println("ForwardVideoFeed: Closing ffmpegInput pipe writer.")
			closer.Close()
		}
	}()
	log.Println("ForwardVideoFeed: Goroutine started.")

	for {
		if stream.Context().Err() != nil {
			log.Printf("ForwardVideoFeed: Stream context cancelled before Recv. Error: %v", stream.Context().Err())
			return
		}

		frame, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				log.Println("ForwardVideoFeed: Video stream EOF received from server.")
			} else {
				s, ok := status.FromError(err)
				if ok && (s.Code() == codes.Canceled) {
					log.Printf("ForwardVideoFeed: Stream cancelled (gRPC status Canceled): %v", err)
				} else {
					log.Printf("ForwardVideoFeed: Error receiving video frame from server: %v", err)
				}
			}
			return
		}

		// Check for server-sent error message
		if errMsg := frame.GetErrorMessage(); errMsg != "" {
			log.Printf("Error from server on video stream: %s", errMsg)
			if mainWindow != nil { // mainWindow is the global variable from client/file_system.go
				dialog.ShowError(fmt.Errorf("Video stream error from host: %s", errMsg), mainWindow)
			} else {
				log.Println("Cannot show video error dialog: global mainWindow is nil.")
			}
			return // Stop processing video
		}

		videoChunk := frame.GetData()
		if videoChunk == nil || len(videoChunk) == 0 {
			continue
		}

		_, writeErr := ffmpegInput.Write(videoChunk)
		if writeErr != nil {
			log.Printf("ForwardVideoFeed: Error writing video chunk to FFmpeg input pipe: %v", writeErr)
			return
		}
	}
}
