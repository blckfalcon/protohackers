package challenge6

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"slices"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/blckfalcon/protohackers/challenges"
)

const (
	// Message types
	ErrorMsg         = 0x10
	PlateMsg         = 0x20
	TicketMsg        = 0x21
	WantHeartbeatMsg = 0x40
	HeartbeatMsg     = 0x41
	IAmCameraMsg     = 0x80
	IAmDispatcherMsg = 0x81
)

type Plate = string
type Road = uint16

// Observation represents a plate observation by a camera
type Observation struct {
	Plate     string
	Timestamp uint32
}

// Camera represents a speed camera
type Camera struct {
	Road  Road
	Mile  uint16
	Limit uint16
	Conn  net.Conn
}

// Dispatcher represents a ticket dispatcher
type Dispatcher struct {
	Roads []Road
	Conn  net.Conn
}

// RoadObservation tracks observations for a specific road and plate
type RoadObservation struct {
	Road      Road
	Mile      uint16
	Timestamp uint32
}

// TicketInfo holds information for generating tickets
type TicketInfo struct {
	Plate      Plate
	Road       Road
	Mile1      uint16
	Timestamp1 uint32
	Mile2      uint16
	Timestamp2 uint32
	Speed      uint16
}

type CarObservation struct {
	Plate     Plate
	Road      Road
	Timestamp uint32
}

// Challenge implements the Challenge interface
type Challenge struct {
	Address string
}

// Name returns the name of the challenge
func (c *Challenge) Name() string {
	return "Speed Enforcement"
}

// Solve implements the solution for the challenge
func (c *Challenge) Solve(ctx context.Context) error {
	listener, err := net.Listen("tcp", c.Address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", c.Address, err)
	}
	defer listener.Close()

	fmt.Printf("Speed enforcement server listening on %s...\n", c.Address)
	fmt.Println("Press Ctrl+C to stop the server")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	// Server state
	server := &Server{
		cameras:        make(map[net.Conn]*Camera),
		dispatchers:    make(map[net.Conn]*Dispatcher),
		observations:   make(map[Plate][]RoadObservation),
		ticketed:       make(map[CarObservation]bool),
		pendingTickets: make(map[Road][]TicketInfo),
		mu:             sync.RWMutex{},
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					if opErr, ok := err.(*net.OpError); ok && opErr.Op == "accept" {
						return
					}
					log.Printf("Error accepting connection: %v", err)
					continue
				}

				go server.handleConnection(ctx, conn)
			}
		}
	}()

	select {
	case <-shutdownChan:
		fmt.Println("\nShutting down speed enforcement server...")
	case <-ctx.Done():
		fmt.Println("\nContext cancelled, shutting down speed enforcement server...")
	}

	return nil
}

// Server holds the state for the speed enforcement system
type Server struct {
	cameras        map[net.Conn]*Camera
	dispatchers    map[net.Conn]*Dispatcher
	observations   map[Plate][]RoadObservation
	ticketed       map[CarObservation]bool
	pendingTickets map[Road][]TicketInfo
	mu             sync.RWMutex
}

// handleConnection processes a single client connection
func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	clientAddr := conn.RemoteAddr().String()
	fmt.Printf("Client connected: %s\n", clientAddr)

	// Track if client has identified itself
	identified := false
	var clientCamera *Camera
	var wantHeartbeat bool
	var heartbeatInterval uint32
	var heartbeatTicker *time.Ticker
	heartbeatDone := make(chan bool)

	cleanup := func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		if cam, exists := s.cameras[conn]; exists {
			delete(s.cameras, conn)
			fmt.Printf("Removed camera at mile %d on road %d\n", cam.Mile, cam.Road)
		}

		if disp, exists := s.dispatchers[conn]; exists {
			delete(s.dispatchers, conn)
			fmt.Printf("Removed dispatcher for roads %v\n", disp.Roads)
		}

		if heartbeatTicker != nil {
			heartbeatTicker.Stop()
		}

		select {
		case heartbeatDone <- true:
		default:
		}
	}

	defer cleanup()

	reader := bufio.NewReader(conn)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msgTypeByte, err := reader.ReadByte()
			if err != nil {
				if err != io.EOF {
					log.Printf("Error reading from %s: %v", clientAddr, err)
				}
				return
			}

			switch msgTypeByte {
			case IAmCameraMsg:
				if identified {
					s.sendError(conn, "already identified")
					return
				}
				identified = true

				camera, err := s.handleIAmCamera(reader, conn)
				if err != nil {
					s.sendError(conn, err.Error())
					return
				}
				clientCamera = camera

				s.mu.Lock()
				s.cameras[conn] = camera
				s.mu.Unlock()

				fmt.Printf("Camera registered: Road %d, Mile %d, Limit %d\n", camera.Road, camera.Mile, camera.Limit)

			case IAmDispatcherMsg:
				if identified {
					s.sendError(conn, "already identified")
					return
				}
				identified = true

				dispatcher, err := s.handleIAmDispatcher(reader, conn)
				if err != nil {
					s.sendError(conn, err.Error())
					return
				}

				s.mu.Lock()
				s.dispatchers[conn] = dispatcher

				// Check for pending tickets for this dispatcher's roads
				for _, road := range dispatcher.Roads {
					if pending, exists := s.pendingTickets[road]; exists && len(pending) > 0 {
						fmt.Printf("Sending %d pending tickets for road %d\n", len(pending), road)
						for _, ticket := range pending {
							var dispatcherConn net.Conn
							for conn, disp := range s.dispatchers {
								if slices.Contains(disp.Roads, road) {
									dispatcherConn = conn
								}
								if dispatcherConn != nil {
									break
								}
							}

							if dispatcherConn != nil {
								if err := s.sendTicketMessage(dispatcherConn, ticket); err != nil {
									fmt.Printf("Error sending pending ticket: %v\n", err)
								}
							} else {
								// This shouldn't happen since we just added a dispatcher for this road
								fmt.Printf("No dispatcher found for road %d when sending pending ticket\n", road)
							}
						}
						delete(s.pendingTickets, road)
					}
				}

				s.mu.Unlock()

				fmt.Printf("Dispatcher registered for roads: %v\n", dispatcher.Roads)

			case PlateMsg:
				if !identified || clientCamera == nil {
					s.sendError(conn, "not a camera")
					return
				}

				observation, err := s.handlePlate(reader)
				if err != nil {
					s.sendError(conn, err.Error())
					return
				}

				roadObservation := RoadObservation{
					Road:      clientCamera.Road,
					Mile:      clientCamera.Mile,
					Timestamp: observation.Timestamp,
				}

				s.mu.Lock()
				s.observations[observation.Plate] = append(s.observations[observation.Plate], roadObservation)

				tickets := s.checkForSpeeding(observation.Plate, s.observations[observation.Plate], clientCamera.Limit)
				s.mu.Unlock()

				for _, ticket := range tickets {
					s.sendTicket(ticket)
				}

			case WantHeartbeatMsg:
				if wantHeartbeat {
					s.sendError(conn, "heartbeat already requested")
					return
				}
				wantHeartbeat = true

				interval, err := s.handleWantHeartbeat(reader)
				if err != nil {
					s.sendError(conn, err.Error())
					return
				}
				heartbeatInterval = interval

				if heartbeatInterval > 0 {
					heartbeatTicker = time.NewTicker(time.Duration(heartbeatInterval) * 100 * time.Millisecond)
				}
				if wantHeartbeat && heartbeatInterval > 0 && heartbeatTicker != nil {
					go func() {
						for {
							select {
							case <-heartbeatDone:
								return
							case <-heartbeatTicker.C:
								if err := s.sendHeartbeat(conn); err != nil {
									fmt.Printf("Error sending heartbeat to %s: %v\n", clientAddr, err)
									return
								}
							}
						}
					}()
				}

			default:
				s.sendError(conn, "unknown message type")
				return
			}
		}
	}
}

// handleIAmCamera processes an IAmCamera message
func (s *Server) handleIAmCamera(reader *bufio.Reader, conn net.Conn) (*Camera, error) {
	// Read road (u16)
	roadBytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, roadBytes); err != nil {
		return nil, fmt.Errorf("failed to read road: %v", err)
	}
	road := binary.BigEndian.Uint16(roadBytes)

	// Read mile (u16)
	mileBytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, mileBytes); err != nil {
		return nil, fmt.Errorf("failed to read mile: %v", err)
	}
	mile := binary.BigEndian.Uint16(mileBytes)

	// Read limit (u16)
	limitBytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, limitBytes); err != nil {
		return nil, fmt.Errorf("failed to read limit: %v", err)
	}
	limit := binary.BigEndian.Uint16(limitBytes)

	return &Camera{
		Road:  road,
		Mile:  mile,
		Limit: limit,
		Conn:  conn,
	}, nil
}

// handleIAmDispatcher processes an IAmDispatcher message
func (s *Server) handleIAmDispatcher(reader *bufio.Reader, conn net.Conn) (*Dispatcher, error) {
	// Read numroads (u8)
	numRoadsByte, err := reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read numroads: %v", err)
	}
	numRoads := int(numRoadsByte)

	// Read roads ([u16])
	roads := make([]uint16, numRoads)
	for i := range numRoads {
		roadBytes := make([]byte, 2)
		if _, err := io.ReadFull(reader, roadBytes); err != nil {
			return nil, fmt.Errorf("failed to read road %d: %v", i, err)
		}
		roads[i] = binary.BigEndian.Uint16(roadBytes)
	}

	return &Dispatcher{
		Roads: roads,
		Conn:  conn,
	}, nil
}

// handlePlate processes a Plate message
func (s *Server) handlePlate(reader *bufio.Reader) (*Observation, error) {
	// Read plate length (u8)
	plateLenByte, err := reader.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("failed to read plate length: %v", err)
	}
	plateLen := int(plateLenByte)

	// Read plate (str)
	plateBytes := make([]byte, plateLen)
	if _, err := io.ReadFull(reader, plateBytes); err != nil {
		return nil, fmt.Errorf("failed to read plate: %v", err)
	}
	plate := string(plateBytes)

	// Read timestamp (u32)
	timestampBytes := make([]byte, 4)
	if _, err := io.ReadFull(reader, timestampBytes); err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %v", err)
	}
	timestamp := binary.BigEndian.Uint32(timestampBytes)

	return &Observation{
		Plate:     plate,
		Timestamp: timestamp,
	}, nil
}

// handleWantHeartbeat processes a WantHeartbeat message
func (s *Server) handleWantHeartbeat(reader *bufio.Reader) (uint32, error) {
	intervalBytes := make([]byte, 4)
	if _, err := io.ReadFull(reader, intervalBytes); err != nil {
		return 0, fmt.Errorf("failed to read interval: %v", err)
	}
	interval := binary.BigEndian.Uint32(intervalBytes)

	return interval, nil
}

// checkForSpeeding checks if there are any speeding violations
func (s *Server) checkForSpeeding(plate string, observations []RoadObservation, limit uint16) []TicketInfo {
	var tickets []TicketInfo

	roadObs := make(map[Road][]RoadObservation)
	for _, obs := range observations {
		roadObs[obs.Road] = append(roadObs[obs.Road], obs)
	}

	for roadNum, obsList := range roadObs {
		sort.Slice(obsList, func(i, j int) bool {
			return obsList[i].Timestamp < obsList[j].Timestamp
		})

		for i := 0; i < len(obsList)-1; i++ {
			for j := i + 1; j < len(obsList); j++ {
				obs1 := obsList[i]
				obs2 := obsList[j]

				if obs1.Mile == obs2.Mile {
					continue
				}

				distance := float64(obs2.Mile) - float64(obs1.Mile)
				timeDiff := float64(obs2.Timestamp) - float64(obs1.Timestamp)

				if timeDiff == 0 {
					continue
				}

				speed := math.Abs(distance / (timeDiff / 3600.0))

				if speed >= float64(limit)+0.5 {
					if !s.hasTicketForCarObservation(plate, roadNum, obs1.Timestamp, obs2.Timestamp) {
						ticket := TicketInfo{
							Plate:      plate,
							Road:       roadNum,
							Mile1:      obs1.Mile,
							Timestamp1: obs1.Timestamp,
							Mile2:      obs2.Mile,
							Timestamp2: obs2.Timestamp,
							Speed:      uint16(speed * 100),
						}
						tickets = append(tickets, ticket)
						s.recordTicketForCarObservation(plate, roadNum, obs1.Timestamp, obs2.Timestamp)
					}
				}
			}
		}
	}

	return tickets
}

// hasTicketForObservationPair checks if we've already issued a ticket for a specific observation pair
func (s *Server) hasTicketForCarObservation(plate string, road uint16, timestamp1, timestamp2 uint32) bool {
	cO1 := CarObservation{Plate: plate, Road: road, Timestamp: timestamp1 / 86400}
	cO2 := CarObservation{Plate: plate, Road: road, Timestamp: timestamp2 / 86400}
	return s.ticketed[cO1] || s.ticketed[cO2]
}

// recordTicketForObservationPair records that we've issued a ticket for a specific observation pair
func (s *Server) recordTicketForCarObservation(plate string, road uint16, timestamp1, timestamp2 uint32) {
	cO1 := CarObservation{Plate: plate, Road: road, Timestamp: timestamp1 / 86400}
	cO2 := CarObservation{Plate: plate, Road: road, Timestamp: timestamp2 / 86400}
	s.ticketed[cO1] = true
	s.ticketed[cO2] = true
}

// sendTicket sends a ticket to an appropriate dispatcher
func (s *Server) sendTicket(ticket TicketInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var dispatcherConn net.Conn
	for conn, dispatcher := range s.dispatchers {
		if slices.Contains(dispatcher.Roads, ticket.Road) {
			dispatcherConn = conn
		}
		if dispatcherConn != nil {
			break
		}
	}

	if dispatcherConn == nil {
		// No dispatcher found, store the ticket for later delivery
		fmt.Printf("No dispatcher found for road %d, storing ticket\n", ticket.Road)
		s.pendingTickets[ticket.Road] = append(s.pendingTickets[ticket.Road], ticket)
		return
	}

	if err := s.sendTicketMessage(dispatcherConn, ticket); err != nil {
		fmt.Printf("Error sending ticket: %v\n", err)
	}
}

// sendTicketMessage sends a Ticket message to a dispatcher
func (s *Server) sendTicketMessage(conn net.Conn, ticket TicketInfo) error {
	msg := make([]byte, 0)

	msg = append(msg, TicketMsg)

	// Plate length + plate
	msg = append(msg, byte(len(ticket.Plate)))
	msg = append(msg, []byte(ticket.Plate)...)

	// Road (u16)
	roadBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(roadBytes, ticket.Road)
	msg = append(msg, roadBytes...)

	// Mile1 (u16)
	mile1Bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(mile1Bytes, ticket.Mile1)
	msg = append(msg, mile1Bytes...)

	// Timestamp1 (u32)
	timestamp1Bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(timestamp1Bytes, ticket.Timestamp1)
	msg = append(msg, timestamp1Bytes...)

	// Mile2 (u16)
	mile2Bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(mile2Bytes, ticket.Mile2)
	msg = append(msg, mile2Bytes...)

	// Timestamp2 (u32)
	timestamp2Bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(timestamp2Bytes, ticket.Timestamp2)
	msg = append(msg, timestamp2Bytes...)

	// Speed (u16)
	speedBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(speedBytes, ticket.Speed)
	msg = append(msg, speedBytes...)

	_, err := conn.Write(msg)
	return err
}

// sendHeartbeat sends a Heartbeat message
func (s *Server) sendHeartbeat(conn net.Conn) error {
	_, err := conn.Write([]byte{HeartbeatMsg})
	return err
}

// sendError sends an Error message and closes the connection
func (s *Server) sendError(conn net.Conn, msg string) error {
	response := make([]byte, 0)

	response = append(response, ErrorMsg)

	response = append(response, byte(len(msg)))
	response = append(response, []byte(msg)...)

	_, err := conn.Write(response)
	return err
}

func init() {
	challenges.Register(6, &Challenge{Address: ":5001"})
}
