package main

import (
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/go-vgo/robotgo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "control_grpc/gen/proto"
	"control_grpc/server/screen"
)

func (s *server) GetFeed(stream pb.RemoteControlService_GetFeedServer) error {
	serverWidth, serverHeight := robotgo.GetScreenSize()
	log.Printf("Server screen dimensions: %dx%d", serverWidth, serverHeight)

	var capture *screen.ScreenCapture
	var err error
	videoCaptureActive := false

	capture, err = screen.NewScreenCapture()
	if err != nil {
		log.Printf("Error initializing screen capture: %v", err)
		errMsg := fmt.Sprintf("Failed to initialize screen capture: %v", err)
		if sendErr := stream.Send(&pb.FeedResponse{
			ErrorMessage: errMsg,
		}); sendErr != nil {
			log.Printf("Error sending screen capture init failure message to client: %v", sendErr)
		}
		videoCaptureActive = false
		// Do not return; allow input events to proceed.
	} else {
		log.Println("Screen capture initialized successfully.")
		videoCaptureActive = true
		defer capture.Close()
	}

	reqMsgInit, err := stream.Recv()
	if err != nil {
		if err == io.EOF {
			log.Println("Client closed stream before init.")
			return nil
		}
		log.Printf("Failed to receive initial message: %v", err)
		return status.Errorf(codes.InvalidArgument, "Failed to receive initial message: %v", err)
	}
	log.Printf("Received init message from client: Width=%d, Height=%d", reqMsgInit.GetClientWidth(), reqMsgInit.GetClientHeight())

	scaleX, scaleY := getScaleFactors(serverWidth, serverHeight, reqMsgInit)
	log.Printf("Calculated scale factors: ScaleX=%.2f, ScaleY=%.2f", scaleX, scaleY)

	inputEvents := make(chan *pb.FeedRequest, 120)
	go handleInputEvents(s, inputEvents, scaleX, scaleY)

	errChan := make(chan error, 1)
	go func() {
		errChan <- receiveInputEvents(stream, inputEvents)
	}()

	if videoCaptureActive && capture != nil {
		log.Println("Starting screen feed sender goroutine.")
		go func() {
			feedErr := sendScreenFeed(stream, capture)
			if feedErr != nil {
				log.Printf("sendScreenFeed goroutine exited with error: %v", feedErr)
			} else {
				log.Println("sendScreenFeed goroutine exited cleanly.")
			}
		}()
	} else {
		log.Println("Video capture is not active; not starting screen feed sender.")
	}

	receiveErr := <-errChan
	close(inputEvents) // Close inputEvents to stop handleInputEvents and related goroutines
	log.Printf("GetFeed: receiveInputEvents goroutine finished with error: %v", receiveErr)

	if receiveErr != nil {
		// Check for specific gRPC status codes that indicate a client-side cancellation or normal stream end
		s, ok := status.FromError(receiveErr)
		if ok && (s.Code() == codes.Canceled || s.Code() == codes.Unavailable) {
			log.Printf("GetFeed: Client stream was canceled or became unavailable: %v", receiveErr)
			return nil // Considered a normal termination from client side
		}
		if receiveErr == io.EOF {
			log.Println("GetFeed: Client closed the stream (EOF).")
			return nil // Normal termination
		}
		log.Printf("GetFeed: Returning with error from receiveInputEvents: %v", receiveErr)
		return receiveErr // Propagate other errors
	}

	log.Println("GetFeed: Exiting cleanly.")
	return nil
}

func getScaleFactors(serverWidth, serverHeight int, reqMsgInit *pb.FeedRequest) (float32, float32) {
	if reqMsgInit.GetClientWidth() == 0 || reqMsgInit.GetClientHeight() == 0 {
		log.Println("Client width or height is zero, using 1.0 for scale factors.")
		return 1.0, 1.0
	}
	scaleX := float32(serverWidth) / float32(reqMsgInit.GetClientWidth())
	scaleY := float32(serverHeight) / float32(reqMsgInit.GetClientHeight())
	return scaleX, scaleY
}

func mapFyneKeyToRobotGo(fyneKeyName string) (key string, isSpecial bool) {
	switch fyneKeyName {
	case "Return", "Enter":
		return "enter", true
	case "Space":
		return "space", true
	case "Backspace":
		return "backspace", true
	case "Delete":
		return "delete", true
	case "Tab":
		return "tab", true
	case "Escape":
		return "escape", true
	case "Up":
		return "up", true
	case "Down":
		return "down", true
	case "Left":
		return "left", true
	case "Right":
		return "right", true
	case "Home":
		return "home", true
	case "End":
		return "end", true
	case "PageUp":
		return "pageup", true
	case "PageDown":
		return "pagedown", true
	case "ShiftL", "LeftShift", "ShiftR", "RightShift":
		return "shift", true
	case "ControlL", "LeftControl", "ControlR", "RightControl":
		return "ctrl", true
	case "AltL", "LeftAlt", "AltR", "RightAlt", "Menu":
		return "alt", true
	case "SuperL", "LeftSuper", "SuperR", "RightSuper", "MetaL", "MetaR":
		return "cmd", true
	case "F1":
		return "f1", true
	case "F2":
		return "f2", true
	case "F3":
		return "f3", true
	case "F4":
		return "f4", true
	case "F5":
		return "f5", true
	case "F6":
		return "f6", true
	case "F7":
		return "f7", true
	case "F8":
		return "f8", true
	case "F9":
		return "f9", true
	case "F10":
		return "f10", true
	case "F11":
		return "f11", true
	case "F12":
		return "f12", true
	case "Num0", "Num1", "Num2", "Num3", "Num4", "Num5", "Num6", "Num7", "Num8", "Num9":
		return strings.ToLower(strings.TrimPrefix(fyneKeyName, "Num")), true
	case "NumLock":
		return "numlock", true
	case "NumEnter":
		return "enter", true
	case "NumAdd", "NumpadAdd":
		return "+", true
	case "NumSubtract", "NumpadSubtract":
		return "-", true
	case "NumMultiply", "NumpadMultiply":
		return "*", true
	case "NumDivide", "NumpadDivide":
		return "/", true
	case "NumDecimal", "NumpadDecimal":
		return ".", true
	default:
		if strings.HasPrefix(fyneKeyName, "Key") && len(fyneKeyName) == 4 {
			char := fyneKeyName[3:]
			if len(char) == 1 && ((char[0] >= 'A' && char[0] <= 'Z') || (char[0] >= '0' && char[0] <= '9')) {
				return strings.ToLower(char), false
			}
		}
		if len(fyneKeyName) == 1 {
			return strings.ToLower(fyneKeyName), false
		}
		if fyneKeyName != "" {
			log.Printf("Unhandled Fyne key name for mapping: '%s'", fyneKeyName)
		}
		return strings.ToLower(fyneKeyName), false
	}
}

func handleInputEvents(s *server, inputEvents chan *pb.FeedRequest, scaleX, scaleY float32) {
	log.Println("Input event handler goroutine started.")
	defer log.Println("Input event handler goroutine stopped.")

	for reqMsg := range inputEvents {

		if reqMsg.Message == "mouse_event" && !s.allowMouseControl {

			isBatched := false
			if reqMsg.GetMouseEventType() == "batched_mouse_moves" {

				if reqMsg.GetBatchedMouseMoves() != nil && len(reqMsg.GetBatchedMouseMoves()) > 0 {
					isBatched = true
				}
			}
			log.Printf("Mouse event (type: %s, batched: %t) ignored: Mouse control denied by host permissions.", reqMsg.GetMouseEventType(), isBatched)
			continue
		}

		switch reqMsg.Message {
		case "mouse_event":
			eventType := reqMsg.GetMouseEventType()
			mouseBtn := reqMsg.GetMouseBtn()

			if eventType == "batched_mouse_moves" {

				batchedMoves := reqMsg.GetBatchedMouseMoves()
				if len(batchedMoves) > 0 {
					log.Printf("Received and processing 'batched_mouse_moves' with %d points.", len(batchedMoves))
					for _, point := range batchedMoves {
						if point == nil {
							continue
						}

						serverX := int(float32(point.X) * scaleX)
						serverY := int(float32(point.Y) * scaleY)
						robotgo.Move(serverX, serverY)

					}
				} else {
					log.Printf("'batched_mouse_moves' received, but the batch contains no points.")
				}
			} else {

				serverX := int(float32(reqMsg.GetMouseX()) * scaleX)
				serverY := int(float32(reqMsg.GetMouseY()) * scaleY)
				robotgo.Move(serverX, serverY)

				if eventType == "down" {
					robotgo.MouseDown(mouseBtn)
				} else if eventType == "up" {
					robotgo.MouseUp(mouseBtn)
				} else if eventType == "scroll" {
					scrollX := reqMsg.GetScrollX()
					scrollY := reqMsg.GetScrollY()

					if scrollX > 0 {
						robotgo.ScrollDir(int(scrollX), "right")
					} else if scrollX < 0 {
						robotgo.ScrollDir(int(-scrollX), "left")
					}
					if scrollY > 0 {
						robotgo.ScrollDir(int(scrollY), "down")
					} else if scrollY < 0 {
						robotgo.ScrollDir(int(-scrollY), "up")
					}
					log.Printf("Handled scroll event: dX=%.2f, dY=%.2f", scrollX, scrollY)
				} else if eventType == "move" {

				} else if eventType != "in" && eventType != "out" {

					log.Printf("Received unhandled mouse event type: %s", eventType)
				}
			}

		case "keyboard_event":
			log.Printf("DEBUG: [handleInputEvents] Forwarding to processKeyboardInput. Type: '%s', KeyName: '%s', KeyChar: '%s'", reqMsg.GetKeyboardEventType(), reqMsg.GetKeyName(), reqMsg.GetKeyCharStr())
			processKeyboardInput(reqMsg)
		default:
			log.Printf("Unknown input event message type: %s", reqMsg.Message)
		}
	}
}

func processKeyboardInput(reqMsg *pb.FeedRequest) {
	kbEventType := reqMsg.GetKeyboardEventType()
	fyneKeyName := reqMsg.GetKeyName()
	keyChar := reqMsg.GetKeyCharStr()

	log.Printf("Received KeyboardEvent: Type='%s', FyneKeyName='%s', KeyChar='%s', Modifiers: Shift[%t], Ctrl[%t], Alt[%t], Super[%t]",
		kbEventType, fyneKeyName, keyChar, reqMsg.GetModifierShift(), reqMsg.GetModifierCtrl(), reqMsg.GetModifierAlt(), reqMsg.GetModifierSuper())

	robotgoKeyName, isSpecial := mapFyneKeyToRobotGo(fyneKeyName)
	if fyneKeyName != "" {
		log.Printf("Mapped FyneKeyName '%s' to robotgoKeyName '%s' (isSpecial: %t)", fyneKeyName, robotgoKeyName, isSpecial)
	}

	if kbEventType == "keydown" && robotgoKeyName == "delete" && reqMsg.GetModifierCtrl() && reqMsg.GetModifierAlt() {
		log.Println("Action: Simulating Ctrl+Alt+Delete")
		robotgo.KeyToggle("ctrl", "down")
		robotgo.KeyToggle("alt", "down")
		robotgo.KeyTap("delete")
		robotgo.KeyToggle("alt", "up")
		robotgo.KeyToggle("ctrl", "up")
	} else {
		switch kbEventType {
		case "keydown":
			if robotgoKeyName != "" {
				isModifierKey := robotgoKeyName == "shift" || robotgoKeyName == "ctrl" || robotgoKeyName == "alt" || robotgoKeyName == "cmd"
				if isModifierKey {
					log.Printf("Action: Modifier '%s' pressed down", robotgoKeyName)
					robotgo.KeyToggle(robotgoKeyName, "down")
				} else if isSpecial {
					log.Printf("Action: Tapping special key '%s'", robotgoKeyName)
					robotgo.KeyTap(robotgoKeyName)
				} else {
					log.Printf("Action: Tapping key '%s'", robotgoKeyName)
					robotgo.KeyTap(robotgoKeyName)
				}
			} else if keyChar != "" {
				log.Printf("Action: Typing character from keyChar on keydown '%s'", keyChar)
				robotgo.TypeStr(keyChar)
			} else {
				log.Printf("Action: Ignoring keydown event with empty robotgoKeyName and KeyChar.")
			}

		case "keyup":
			if robotgoKeyName != "" {
				isModifierKey := robotgoKeyName == "shift" || robotgoKeyName == "ctrl" || robotgoKeyName == "alt" || robotgoKeyName == "cmd"
				if isModifierKey {
					log.Printf("Action: Modifier '%s' released", robotgoKeyName)
					robotgo.KeyToggle(robotgoKeyName, "up")
				} else {

					log.Printf("Action: Ignoring non-modifier keyup for '%s' (handled by KeyTap on keydown)", robotgoKeyName)
				}
			} else {
				log.Printf("Action: Ignoring keyup event with empty robotgoKeyName.")
			}

		case "keychar":
			if keyChar != "" {
				log.Printf("Action: Typing character from keychar event '%s'", keyChar)
				robotgo.TypeStr(keyChar)
			} else {
				log.Printf("Action: Ignoring keychar event with empty KeyChar.")
			}
		default:
			log.Printf("Action: Unhandled keyboard event type: '%s'", kbEventType)
		}
	}
}

func receiveInputEvents(stream pb.RemoteControlService_GetFeedServer, inputEvents chan *pb.FeedRequest) error {
	log.Println("Input event receiver goroutine started.")
	defer log.Println("Input event receiver goroutine stopped.")
	// Not closing inputEvents here anymore; GetFeed will manage it.

	for {
		reqMsg, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				log.Println("Client closed the stream (EOF in receiveInputEvents).")
				return err // Propagate EOF
			}
			s, ok := status.FromError(err)
			if ok && (s.Code() == codes.Canceled || s.Code() == codes.Unavailable) {
				log.Printf("Stream canceled or unavailable in receiveInputEvents: %v", err)
				return err // Propagate cancellation/unavailability
			}
			log.Printf("Error receiving input event from stream: %v", err)
			return err // Propagate other errors
		}

		select {
		case inputEvents <- reqMsg:

		default:
			log.Println("Input event channel full, dropping event.")
		}
	}
}

func sendScreenFeed(stream pb.RemoteControlService_GetFeedServer, capture *screen.ScreenCapture) error {
	log.Println("Screen feed sender goroutine started.")
	defer log.Println("Screen feed sender goroutine stopped.")

	frameBuffer := make([]byte, 2*1024*1024)
	ticker := time.NewTicker(time.Second / 30)
	defer ticker.Stop()

	var frameCounter int32 = 0
	for {
		select {
		case <-ticker.C:
			n, err := capture.ReadFrame(frameBuffer)
			if err != nil {
				if err == io.EOF {
					log.Println("Screen capture source reported EOF.")
					return status.Errorf(codes.Internal, "Screen capture source EOF")
				}
				log.Printf("Error reading frame from screen capture: %v", err)
				continue
			}
			if n == 0 {
				continue
			}

			err = stream.Send(&pb.FeedResponse{
				Data:        frameBuffer[:n],
				FrameNumber: frameCounter,
				Timestamp:   time.Now().UnixNano(),
				ContentType: "video/mp2t",
				HwAccel:     screen.Accel,
			})
			if err != nil {
				s, ok := status.FromError(err)
				if ok && (s.Code() == codes.Canceled || s.Code() == codes.Unavailable) {
					log.Printf("Client disconnected or stream unavailable during send: %v", err)
					return nil
				}
				log.Printf("Error sending frame to client: %v", err)
				return status.Errorf(codes.Internal, "Failed to send frame: %v", err)
			}
			frameCounter++
		case <-stream.Context().Done():
			log.Printf("Stream context done (client likely disconnected): %v", stream.Context().Err())
			return nil
		}
	}
}
