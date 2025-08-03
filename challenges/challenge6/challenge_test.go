package challenge6

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestHeartbeat(t *testing.T) {
	challenge := &Challenge{
		Address: GetAvailablePort(t),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverReady := make(chan struct{})

	go func() {
		close(serverReady)

		if err := challenge.Solve(ctx); err != nil {
			select {
			case <-ctx.Done():
				// This is expected during shutdown
				return
			default:
				t.Errorf("Server error: %v", err)
			}
		}
	}()
	<-serverReady

	conn, err := net.Dial("tcp", challenge.Address)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Send WantHeartbeat message with interval of 10
	// Meaning a message every 1 seconds
	wantHeartbeatMsg := make([]byte, 5)
	wantHeartbeatMsg[0] = WantHeartbeatMsg
	binary.BigEndian.PutUint32(wantHeartbeatMsg[1:], 10)

	_, err = conn.Write(wantHeartbeatMsg)
	if err != nil {
		t.Fatalf("Failed to send WantHeartbeat message: %v", err)
	}

	reader := bufio.NewReader(conn)

	// Set a timeout for receiving heartbeats (should get 3 in 3 seconds)
	heartbeatCount := 0
	heartbeatExpected := 3
	heartbeatTimeout := time.After(4 * time.Second)
	testTimeout := time.After(5 * time.Second)

	for {
		select {
		case <-testTimeout:
			t.Fatal("Test timed out waiting for heartbeats")
			return
		case <-heartbeatTimeout:
			if heartbeatCount < heartbeatExpected {
				t.Fatalf("Expected at least %d heartbeats within 3 seconds, got %d", heartbeatExpected, heartbeatCount)
			}
			// Test passed
			return
		default:
			// Set a read deadline to avoid blocking forever
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))

			msgType, err := reader.ReadByte()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // Just continue waiting
				}
				t.Logf("Error reading from connection: %v", err)
				continue
			}

			if msgType == HeartbeatMsg {
				heartbeatCount++
				t.Logf("Received heartbeat #%d", heartbeatCount)

				if heartbeatCount >= heartbeatExpected {
					t.Logf("Successfully received %d heartbeats", heartbeatCount)
					return
				}
			} else {
				// Log unexpected messages but don't fail the test
				t.Logf("Received unexpected message type: 0x%02x", msgType)
			}
		}
	}
}

func TestTicketStorageDispatcher(t *testing.T) {
	challenge := &Challenge{
		Address: GetAvailablePort(t),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverReady := make(chan struct{})

	go func() {
		close(serverReady)

		if err := challenge.Solve(ctx); err != nil {
			select {
			case <-ctx.Done():
				// This is expected during shutdown
				return
			default:
				t.Errorf("Server error: %v", err)
			}
		}
	}()
	<-serverReady

	cameraConn, err := net.Dial("tcp", challenge.Address)
	if err != nil {
		t.Fatalf("Failed to connect as camera: %v", err)
	}
	defer cameraConn.Close()

	// Register as a camera on road 123 at mile 0 with speed limit 60
	iamCameraMsg := make([]byte, 7)
	iamCameraMsg[0] = IAmCameraMsg
	binary.BigEndian.PutUint16(iamCameraMsg[1:], 123) // road
	binary.BigEndian.PutUint16(iamCameraMsg[3:], 0)   // mile
	binary.BigEndian.PutUint16(iamCameraMsg[5:], 60)  // limit

	_, err = cameraConn.Write(iamCameraMsg)
	if err != nil {
		t.Fatalf("Failed to send IAmCamera message: %v", err)
	}

	// Send a plate observation at mile 0, timestamp 0
	plateMsg := make([]byte, 10)
	plateMsg[0] = PlateMsg
	plateMsg[1] = 4
	plateMsg[2] = 'T'
	plateMsg[3] = 'E'
	plateMsg[4] = 'S'
	plateMsg[5] = 'T'
	binary.BigEndian.PutUint32(plateMsg[6:], 0)

	_, err = cameraConn.Write(plateMsg)
	if err != nil {
		t.Fatalf("Failed to send Plate message: %v", err)
	}

	cameraConn.Close()

	cameraConn2, err := net.Dial("tcp", challenge.Address)
	if err != nil {
		t.Fatalf("Failed to connect as second camera: %v", err)
	}
	defer cameraConn2.Close()

	// Register as a camera on road 123 at mile 60 with speed limit 60
	iamCameraMsg2 := make([]byte, 7)
	iamCameraMsg2[0] = IAmCameraMsg
	binary.BigEndian.PutUint16(iamCameraMsg2[1:], 123) // road
	binary.BigEndian.PutUint16(iamCameraMsg2[3:], 60)  // mile
	binary.BigEndian.PutUint16(iamCameraMsg2[5:], 60)  // limit

	_, err = cameraConn2.Write(iamCameraMsg2)
	if err != nil {
		t.Fatalf("Failed to send IAmCamera message: %v", err)
	}

	// Send the same plate observation at mile 60, timestamp 3000 (50 minutes later)
	plateMsg2 := make([]byte, 10)
	plateMsg2[0] = PlateMsg
	plateMsg2[1] = 4
	plateMsg2[2] = 'T'
	plateMsg2[3] = 'E'
	plateMsg2[4] = 'S'
	plateMsg2[5] = 'T'
	binary.BigEndian.PutUint32(plateMsg2[6:], 3000)

	_, err = cameraConn2.Write(plateMsg2)
	if err != nil {
		t.Fatalf("Failed to send Plate message: %v", err)
	}

	// Give the server time to process and generate a ticket
	time.Sleep(100 * time.Millisecond)

	// Now connect as a dispatcher for road 123
	dispatcherConn, err := net.Dial("tcp", challenge.Address)
	if err != nil {
		t.Fatalf("Failed to connect as dispatcher: %v", err)
	}
	defer dispatcherConn.Close()

	iamDispatcherMsg := make([]byte, 4)
	iamDispatcherMsg[0] = IAmDispatcherMsg
	iamDispatcherMsg[1] = 1                               // numroads
	binary.BigEndian.PutUint16(iamDispatcherMsg[2:], 123) // road

	_, err = dispatcherConn.Write(iamDispatcherMsg)
	if err != nil {
		t.Fatalf("Failed to send IAmDispatcher message: %v", err)
	}

	// Wait for the ticket to be delivered
	reader := bufio.NewReader(dispatcherConn)
	dispatcherConn.SetReadDeadline(time.Now().Add(2 * time.Second))

	msgType, err := reader.ReadByte()
	if err != nil {
		t.Fatalf("Failed to read message from dispatcher: %v", err)
	}

	if msgType != TicketMsg {
		t.Fatalf("Expected TicketMsg (0x%02x), got 0x%02x", TicketMsg, msgType)
	}
}

func TestMultiTickets(t *testing.T) {
	challenge := &Challenge{
		Address: GetAvailablePort(t),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverReady := make(chan struct{})

	go func() {
		close(serverReady)

		if err := challenge.Solve(ctx); err != nil {
			select {
			case <-ctx.Done():
				// This is expected during shutdown
				return
			default:
				t.Errorf("Server error: %v", err)
			}
		}
	}()
	<-serverReady

	// Connect as a dispatcher for road 123 to receive tickets
	dispatcherConn, err := net.Dial("tcp", challenge.Address)
	if err != nil {
		t.Fatalf("Failed to connect as dispatcher: %v", err)
	}
	defer dispatcherConn.Close()

	iamDispatcherMsg := make([]byte, 4)
	iamDispatcherMsg[0] = IAmDispatcherMsg
	iamDispatcherMsg[1] = 1                               // numroads
	binary.BigEndian.PutUint16(iamDispatcherMsg[2:], 123) // road

	_, err = dispatcherConn.Write(iamDispatcherMsg)
	if err != nil {
		t.Fatalf("Failed to send IAmDispatcher message: %v", err)
	}

	reader := bufio.NewReader(dispatcherConn)

	// Set up camera 1 at mile 0
	cameraConn1, err := net.Dial("tcp", challenge.Address)
	if err != nil {
		t.Fatalf("Failed to connect as camera: %v", err)
	}

	// Register as a camera on road 123 at mile 0 with speed limit 60
	iamCameraMsg := make([]byte, 7)
	iamCameraMsg[0] = IAmCameraMsg
	binary.BigEndian.PutUint16(iamCameraMsg[1:], 123) // road
	binary.BigEndian.PutUint16(iamCameraMsg[3:], 0)   // mile
	binary.BigEndian.PutUint16(iamCameraMsg[5:], 60)  // limit

	_, err = cameraConn1.Write(iamCameraMsg)
	if err != nil {
		t.Fatalf("Failed to send IAmCamera message: %v", err)
	}

	// Set up camera 2 at mile 60
	cameraConn2, err := net.Dial("tcp", challenge.Address)
	if err != nil {
		t.Fatalf("Failed to connect as second camera: %v", err)
	}

	// Register as a camera on road 123 at mile 60 with speed limit 60
	iamCameraMsg2 := make([]byte, 7)
	iamCameraMsg2[0] = IAmCameraMsg
	binary.BigEndian.PutUint16(iamCameraMsg2[1:], 123) // road
	binary.BigEndian.PutUint16(iamCameraMsg2[3:], 60)  // mile
	binary.BigEndian.PutUint16(iamCameraMsg2[5:], 60)  // limit

	_, err = cameraConn2.Write(iamCameraMsg2)
	if err != nil {
		t.Fatalf("Failed to send IAmCamera message: %v", err)
	}

	// Test 1: Same day - should generate only 1 ticket
	// Send plate observation at mile 0, timestamp 0 (day 0)
	plateMsg := make([]byte, 10)
	plateMsg[0] = PlateMsg
	plateMsg[1] = 4
	plateMsg[2] = 'T'
	plateMsg[3] = 'E'
	plateMsg[4] = 'S'
	plateMsg[5] = 'T'
	binary.BigEndian.PutUint32(plateMsg[6:], 0)

	_, err = cameraConn1.Write(plateMsg)
	if err != nil {
		t.Fatalf("Failed to send Plate message: %v", err)
	}

	// Send the same plate observation at mile 60, timestamp 3000 (50 minutes later, still day 0)
	plateMsg2 := make([]byte, 10)
	plateMsg2[0] = PlateMsg
	plateMsg2[1] = 4
	plateMsg2[2] = 'T'
	plateMsg2[3] = 'E'
	plateMsg2[4] = 'S'
	plateMsg2[5] = 'T'
	binary.BigEndian.PutUint32(plateMsg2[6:], 3000)

	_, err = cameraConn2.Write(plateMsg2)
	if err != nil {
		t.Fatalf("Failed to send Plate message: %v", err)
	}

	// Give the server time to process and generate the ticket
	time.Sleep(100 * time.Millisecond)

	// Wait for the first ticket to be delivered
	dispatcherConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgType, err := reader.ReadByte()
	if err != nil {
		t.Fatalf("Failed to read message from dispatcher: %v", err)
	}

	if msgType != TicketMsg {
		t.Fatalf("Expected TicketMsg (0x%02x), got 0x%02x", TicketMsg, msgType)
	}

	// Read the ticket data
	plateLen, err := reader.ReadByte()
	if err != nil {
		t.Fatalf("Failed to read plate length: %v", err)
	}
	plateBytes := make([]byte, plateLen)
	if _, err := io.ReadFull(reader, plateBytes); err != nil {
		t.Fatalf("Failed to read plate: %v", err)
	}
	plate := string(plateBytes)

	roadBytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, roadBytes); err != nil {
		t.Fatalf("Failed to read road: %v", err)
	}
	road := binary.BigEndian.Uint16(roadBytes)

	mile1Bytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, mile1Bytes); err != nil {
		t.Fatalf("Failed to read mile1: %v", err)
	}
	mile1 := binary.BigEndian.Uint16(mile1Bytes)

	timestamp1Bytes := make([]byte, 4)
	if _, err := io.ReadFull(reader, timestamp1Bytes); err != nil {
		t.Fatalf("Failed to read timestamp1: %v", err)
	}
	timestamp1 := binary.BigEndian.Uint32(timestamp1Bytes)

	mile2Bytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, mile2Bytes); err != nil {
		t.Fatalf("Failed to read mile2: %v", err)
	}
	mile2 := binary.BigEndian.Uint16(mile2Bytes)

	timestamp2Bytes := make([]byte, 4)
	if _, err := io.ReadFull(reader, timestamp2Bytes); err != nil {
		t.Fatalf("Failed to read timestamp2: %v", err)
	}
	timestamp2 := binary.BigEndian.Uint32(timestamp2Bytes)

	speedBytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, speedBytes); err != nil {
		t.Fatalf("Failed to read speed: %v", err)
	}
	speed := binary.BigEndian.Uint16(speedBytes)

	// Verify the ticket details
	if plate != "TEST" {
		t.Errorf("Expected plate 'TEST', got '%s'", plate)
	}
	if road != 123 {
		t.Errorf("Expected road 123, got %d", road)
	}
	if mile1 != 0 {
		t.Errorf("Expected mile1 0, got %d", mile1)
	}
	if timestamp1 != 0 {
		t.Errorf("Expected timestamp1 0, got %d", timestamp1)
	}
	if mile2 != 60 {
		t.Errorf("Expected mile2 60, got %d", mile2)
	}
	if timestamp2 != 3000 {
		t.Errorf("Expected timestamp2 3000, got %d", timestamp2)
	}
	if speed != 7200 {
		t.Errorf("Expected speed 7200, got %d", speed)
	}

	// Test 2: Different day - should generate another ticket
	// Send plate observation at mile 0, timestamp 86400 (day 1)
	plateMsg3 := make([]byte, 10)
	plateMsg3[0] = PlateMsg
	plateMsg3[1] = 4
	plateMsg3[2] = 'T'
	plateMsg3[3] = 'E'
	plateMsg3[4] = 'S'
	plateMsg3[5] = 'T'
	binary.BigEndian.PutUint32(plateMsg3[6:], 86400)

	_, err = cameraConn1.Write(plateMsg3)
	if err != nil {
		t.Fatalf("Failed to send Plate message: %v", err)
	}

	// Send the same plate observation at mile 60, timestamp 89400 (50 minutes later, still day 1)
	plateMsg4 := make([]byte, 10)
	plateMsg4[0] = PlateMsg
	plateMsg4[1] = 4
	plateMsg4[2] = 'T'
	plateMsg4[3] = 'E'
	plateMsg4[4] = 'S'
	plateMsg4[5] = 'T'
	binary.BigEndian.PutUint32(plateMsg4[6:], 89400)

	_, err = cameraConn2.Write(plateMsg4)
	if err != nil {
		t.Fatalf("Failed to send Plate message: %v", err)
	}

	// Give the server time to process and generate the second ticket
	time.Sleep(100 * time.Millisecond)

	// Wait for the second ticket to be delivered
	dispatcherConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgType, err = reader.ReadByte()
	if err != nil {
		t.Fatalf("Failed to read second message from dispatcher: %v", err)
	}

	if msgType != TicketMsg {
		t.Fatalf("Expected TicketMsg (0x%02x) for second ticket, got 0x%02x", TicketMsg, msgType)
	}

	// Read the second ticket data
	plateLen, err = reader.ReadByte()
	if err != nil {
		t.Fatalf("Failed to read plate length for second ticket: %v", err)
	}
	plateBytes = make([]byte, plateLen)
	if _, err := io.ReadFull(reader, plateBytes); err != nil {
		t.Fatalf("Failed to read plate for second ticket: %v", err)
	}
	plate = string(plateBytes)

	roadBytes = make([]byte, 2)
	if _, err := io.ReadFull(reader, roadBytes); err != nil {
		t.Fatalf("Failed to read road for second ticket: %v", err)
	}
	road = binary.BigEndian.Uint16(roadBytes)

	mile1Bytes = make([]byte, 2)
	if _, err := io.ReadFull(reader, mile1Bytes); err != nil {
		t.Fatalf("Failed to read mile1 for second ticket: %v", err)
	}
	mile1 = binary.BigEndian.Uint16(mile1Bytes)

	timestamp1Bytes = make([]byte, 4)
	if _, err := io.ReadFull(reader, timestamp1Bytes); err != nil {
		t.Fatalf("Failed to read timestamp1 for second ticket: %v", err)
	}
	timestamp1 = binary.BigEndian.Uint32(timestamp1Bytes)

	mile2Bytes = make([]byte, 2)
	if _, err := io.ReadFull(reader, mile2Bytes); err != nil {
		t.Fatalf("Failed to read mile2 for second ticket: %v", err)
	}
	mile2 = binary.BigEndian.Uint16(mile2Bytes)

	timestamp2Bytes = make([]byte, 4)
	if _, err := io.ReadFull(reader, timestamp2Bytes); err != nil {
		t.Fatalf("Failed to read timestamp2 for second ticket: %v", err)
	}
	timestamp2 = binary.BigEndian.Uint32(timestamp2Bytes)

	speedBytes = make([]byte, 2)
	if _, err := io.ReadFull(reader, speedBytes); err != nil {
		t.Fatalf("Failed to read speed: %v", err)
	}
	speed = binary.BigEndian.Uint16(speedBytes)

	// Verify the second ticket details
	if plate != "TEST" {
		t.Errorf("Expected plate 'TEST' for second ticket, got '%s'", plate)
	}
	if road != 123 {
		t.Errorf("Expected road 123 for second ticket, got %d", road)
	}
	if mile1 != 0 {
		t.Errorf("Expected mile1 0 for second ticket, got %d", mile1)
	}
	if timestamp1 != 86400 {
		t.Errorf("Expected timestamp1 86400 for second ticket, got %d", timestamp1)
	}
	if mile2 != 60 {
		t.Errorf("Expected mile2 60 for second ticket, got %d", mile2)
	}
	if timestamp2 != 89400 {
		t.Errorf("Expected timestamp2 89400 for second ticket, got %d", timestamp2)
	}
	if speed != 7200 {
		t.Errorf("Expected speed 7200, got %d", speed)
	}

	// Test 3: Same day again - should NOT generate another ticket
	// Send plate observation at mile 0, timestamp 90000 (still day 1)
	plateMsg5 := make([]byte, 10)
	plateMsg5[0] = PlateMsg
	plateMsg5[1] = 4
	plateMsg5[2] = 'T'
	plateMsg5[3] = 'E'
	plateMsg5[4] = 'S'
	plateMsg5[5] = 'T'
	binary.BigEndian.PutUint32(plateMsg5[6:], 90000)

	_, err = cameraConn1.Write(plateMsg5)
	if err != nil {
		t.Fatalf("Failed to send Plate message: %v", err)
	}

	// Send the same plate observation at mile 60, timestamp 93000 (50 minutes later, still day 1)
	plateMsg6 := make([]byte, 10)
	plateMsg6[0] = PlateMsg
	plateMsg6[1] = 4
	plateMsg6[2] = 'T'
	plateMsg6[3] = 'E'
	plateMsg6[4] = 'S'
	plateMsg6[5] = 'T'
	binary.BigEndian.PutUint32(plateMsg6[6:], 93000)

	_, err = cameraConn2.Write(plateMsg6)
	if err != nil {
		t.Fatalf("Failed to send Plate message: %v", err)
	}

	// Give the server time to process
	time.Sleep(100 * time.Millisecond)

	// Set a short timeout to check if a third ticket is sent
	dispatcherConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

	// Try to read a message - should timeout because no ticket should be sent
	msgType, err = reader.ReadByte()
	if err == nil {
		t.Fatal("Unexpected third ticket received - should only get 1 ticket per day")
	} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		// This is expected - no ticket should be sent
		t.Log("No third ticket received as expected (1 ticket per day limit)")
	} else {
		t.Fatalf("Unexpected error reading from dispatcher: %v", err)
	}

	cameraConn1.Close()
	cameraConn2.Close()
}

func TestMultiCars(t *testing.T) {
	challenge := &Challenge{
		Address: GetAvailablePort(t),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverReady := make(chan struct{})

	go func() {
		close(serverReady)

		if err := challenge.Solve(ctx); err != nil {
			select {
			case <-ctx.Done():
				// This is expected during shutdown
				return
			default:
				t.Errorf("Server error: %v", err)
			}
		}
	}()
	<-serverReady

	// Connect as a dispatcher for road 123 to receive tickets
	dispatcherConn, err := net.Dial("tcp", challenge.Address)
	if err != nil {
		t.Fatalf("Failed to connect as dispatcher: %v", err)
	}
	defer dispatcherConn.Close()

	iamDispatcherMsg := make([]byte, 4)
	iamDispatcherMsg[0] = IAmDispatcherMsg
	iamDispatcherMsg[1] = 1                               // numroads
	binary.BigEndian.PutUint16(iamDispatcherMsg[2:], 123) // road

	_, err = dispatcherConn.Write(iamDispatcherMsg)
	if err != nil {
		t.Fatalf("Failed to send IAmDispatcher message: %v", err)
	}

	reader := bufio.NewReader(dispatcherConn)

	// Set up camera 1 at mile 0
	cameraConn1, err := net.Dial("tcp", challenge.Address)
	if err != nil {
		t.Fatalf("Failed to connect as camera: %v", err)
	}

	// Register as a camera on road 123 at mile 0 with speed limit 60
	iamCameraMsg := make([]byte, 7)
	iamCameraMsg[0] = IAmCameraMsg
	binary.BigEndian.PutUint16(iamCameraMsg[1:], 123) // road
	binary.BigEndian.PutUint16(iamCameraMsg[3:], 0)   // mile
	binary.BigEndian.PutUint16(iamCameraMsg[5:], 60)  // limit

	_, err = cameraConn1.Write(iamCameraMsg)
	if err != nil {
		t.Fatalf("Failed to send IAmCamera message: %v", err)
	}

	// Set up camera 2 at mile 60
	cameraConn2, err := net.Dial("tcp", challenge.Address)
	if err != nil {
		t.Fatalf("Failed to connect as second camera: %v", err)
	}

	// Register as a camera on road 123 at mile 60 with speed limit 60
	iamCameraMsg2 := make([]byte, 7)
	iamCameraMsg2[0] = IAmCameraMsg
	binary.BigEndian.PutUint16(iamCameraMsg2[1:], 123) // road
	binary.BigEndian.PutUint16(iamCameraMsg2[3:], 60)  // mile
	binary.BigEndian.PutUint16(iamCameraMsg2[5:], 60)  // limit

	_, err = cameraConn2.Write(iamCameraMsg2)
	if err != nil {
		t.Fatalf("Failed to send IAmCamera message: %v", err)
	}

	// Test with multiple cars (different plates)
	// Car 1: Plate "TEST"
	// Send plate observation at mile 0, timestamp 0
	plateMsg1 := make([]byte, 10)
	plateMsg1[0] = PlateMsg
	plateMsg1[1] = 4
	plateMsg1[2] = 'T'
	plateMsg1[3] = 'E'
	plateMsg1[4] = 'S'
	plateMsg1[5] = 'T'
	binary.BigEndian.PutUint32(plateMsg1[6:], 0)

	_, err = cameraConn1.Write(plateMsg1)
	if err != nil {
		t.Fatalf("Failed to send Plate message for car 1: %v", err)
	}

	// Car 2: Plate "CAR2"
	// Send plate observation at mile 0, timestamp 100
	plateMsg2 := make([]byte, 10)
	plateMsg2[0] = PlateMsg
	plateMsg2[1] = 4
	plateMsg2[2] = 'C'
	plateMsg2[3] = 'A'
	plateMsg2[4] = 'R'
	plateMsg2[5] = '2'
	binary.BigEndian.PutUint32(plateMsg2[6:], 100)

	_, err = cameraConn1.Write(plateMsg2)
	if err != nil {
		t.Fatalf("Failed to send Plate message for car 2: %v", err)
	}

	// Car 3: Plate "CAR3"
	// Send plate observation at mile 0, timestamp 200
	plateMsg3 := make([]byte, 10)
	plateMsg3[0] = PlateMsg
	plateMsg3[1] = 4
	plateMsg3[2] = 'C'
	plateMsg3[3] = 'A'
	plateMsg3[4] = 'R'
	plateMsg3[5] = '3'
	binary.BigEndian.PutUint32(plateMsg3[6:], 200)

	_, err = cameraConn1.Write(plateMsg3)
	if err != nil {
		t.Fatalf("Failed to send Plate message for car 3: %v", err)
	}

	// Now send observations from camera 2 for all cars
	// Car 1: Plate "TEST" at mile 60, timestamp 3000
	plateMsg4 := make([]byte, 10)
	plateMsg4[0] = PlateMsg
	plateMsg4[1] = 4
	plateMsg4[2] = 'T'
	plateMsg4[3] = 'E'
	plateMsg4[4] = 'S'
	plateMsg4[5] = 'T'
	binary.BigEndian.PutUint32(plateMsg4[6:], 3000)

	_, err = cameraConn2.Write(plateMsg4)
	if err != nil {
		t.Fatalf("Failed to send Plate message for car 1 at camera 2: %v", err)
	}

	// Car 2: Plate "CAR2" at mile 60, timestamp 3100
	plateMsg5 := make([]byte, 10)
	plateMsg5[0] = PlateMsg
	plateMsg5[1] = 4
	plateMsg5[2] = 'C'
	plateMsg5[3] = 'A'
	plateMsg5[4] = 'R'
	plateMsg5[5] = '2'
	binary.BigEndian.PutUint32(plateMsg5[6:], 3100)

	_, err = cameraConn2.Write(plateMsg5)
	if err != nil {
		t.Fatalf("Failed to send Plate message for car 2 at camera 2: %v", err)
	}

	// Car 3: Plate "CAR3" at mile 60, timestamp 3200
	plateMsg6 := make([]byte, 10)
	plateMsg6[0] = PlateMsg
	plateMsg6[1] = 4
	plateMsg6[2] = 'C'
	plateMsg6[3] = 'A'
	plateMsg6[4] = 'R'
	plateMsg6[5] = '3'
	binary.BigEndian.PutUint32(plateMsg6[6:], 3200)

	_, err = cameraConn2.Write(plateMsg6)
	if err != nil {
		t.Fatalf("Failed to send Plate message for car 3 at camera 2: %v", err)
	}

	// Give the server time to process and generate tickets
	time.Sleep(100 * time.Millisecond)

	receivedTickets := make(map[string]bool)

	for i := range 3 {
		// Wait for a ticket to be delivered
		dispatcherConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		msgType, err := reader.ReadByte()
		if err != nil {
			t.Fatalf("Failed to read message %d from dispatcher: %v", i+1, err)
		}

		if msgType != TicketMsg {
			t.Fatalf("Expected TicketMsg (0x%02x) for ticket %d, got 0x%02x", TicketMsg, i+1, msgType)
		}

		plateLen, err := reader.ReadByte()
		if err != nil {
			t.Fatalf("Failed to read plate length for ticket %d: %v", i+1, err)
		}
		plateBytes := make([]byte, plateLen)
		if _, err := io.ReadFull(reader, plateBytes); err != nil {
			t.Fatalf("Failed to read plate for ticket %d: %v", i+1, err)
		}
		plate := string(plateBytes)

		receivedTickets[plate] = true

		_, err = reader.Discard(16) // road (2) + mile1 (2) + timestamp1 (4) + mile2 (2) + timestamp2 (4) + speed (2)
		if err != nil {
			t.Fatalf("Failed to discard remaining ticket data for ticket %d: %v", i+1, err)
		}
	}

	// Verify we received tickets for all three cars
	if !receivedTickets["TEST"] {
		t.Error("Did not receive ticket for car with plate 'TEST'")
	}
	if !receivedTickets["CAR2"] {
		t.Error("Did not receive ticket for car with plate 'CAR2'")
	}
	if !receivedTickets["CAR3"] {
		t.Error("Did not receive ticket for car with plate 'CAR3'")
	}

	if len(receivedTickets) != 3 {
		t.Errorf("Expected 3 tickets, received %d", len(receivedTickets))
	}

	cameraConn1.Close()
	cameraConn2.Close()
}

func GetAvailablePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	return ":" + string(port)
}
