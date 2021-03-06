package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"

	doorservicepb "github.com/0xc0d3d00d/openify/go/proto/doorservice"
	"github.com/0xc0d3d00d/openify/go/sql"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var db *sql.Sql
var doorChannels map[int64]*chan bool

type grpcServer struct{}

func init() {
	var err error
	db, err = sql.New()
	if err != nil {
		log.Panicln(err)
	}

	doorChannels = make(map[int64]*chan bool)
}

func (s *grpcServer) UpdateState(ctx context.Context, in *doorservicepb.UpdateStateRequest) (*doorservicepb.UpdateStateResponse, error) {
	accessLog := sql.AccessLog{
		DoorId: in.Serial,
		State:  in.State,
		UserId: nil,
	}
	err := db.StoreAccessLog(accessLog)
	if err != nil {
		log.Println(err)
		return nil, errors.New("Internal server error")
	}
	log.Printf("stored access log for door %d: %s", in.Serial, doorservicepb.DoorState_name[int32(in.State)])
	return &doorservicepb.UpdateStateResponse{}, nil
}

func (s *grpcServer) AccessStream(in *doorservicepb.AccessStreamRequest, streamServer doorservicepb.DoorService_AccessStreamServer) error {
	doorChannel := make(chan bool, 1)
	doorChannels[in.Serial] = &doorChannel
	log.Printf("door %d connected", in.Serial)
	for {
		<-*(doorChannels[in.Serial])
		err := streamServer.Send(&doorservicepb.AccessStreamResponse{OpenDoor: true})
		log.Printf("sending open command to door %d", in.Serial)
		if err != nil {
			log.Println(err)
			return err
		}
	}
}

type openDoorError int16

// Door service possible errors
const (
	ErrorInvalidRequest      = 400
	ErrorInternalServerError = 500
	ErrorDoorIsInaccessible  = 503
	ErrorSuccess             = 0
)

type openDoorRequest struct {
	DoorSerial int64 `json:"door_serial"`
	UserId     int64 `json:"user_id"`
}
type openDoorResponse struct {
	Success bool          `json:"success"`
	Code    openDoorError `json:"error_code,omitempty"`
	Err     string        `json:"error,omitempty"`
}

func openDoorApi(w http.ResponseWriter, r *http.Request) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)

	parsedReq := &openDoorRequest{}
	err := json.Unmarshal(buf.Bytes(), parsedReq)
	if err != nil || parsedReq.DoorSerial == 0 || parsedReq.UserId == 0 {
		log.Println(err)
		errorResp, _ := json.Marshal(openDoorResponse{Success: false, Code: ErrorInvalidRequest, Err: "Invalid Request"})
		w.WriteHeader(400)
		w.Write(errorResp)
		return
	}

	if _, present := doorChannels[parsedReq.DoorSerial]; !present {
		errorResp, _ := json.Marshal(openDoorResponse{Success: false, Code: ErrorDoorIsInaccessible, Err: "Door is inaccessible"})
		w.WriteHeader(503)
		w.Write(errorResp)
		return
	}

	doorChan := *(doorChannels[parsedReq.DoorSerial])
	doorChan <- true

	accessLog := sql.AccessLog{
		UserId: &parsedReq.UserId,
		DoorId: parsedReq.DoorSerial,
		State:  doorservicepb.DoorState_OPEN,
	}
	err = db.StoreAccessLog(accessLog)
	if err != nil {
		errorResp, _ := json.Marshal(openDoorResponse{Success: false, Code: ErrorInternalServerError, Err: "Internal Server Error"})
		w.WriteHeader(500)
		w.Write(errorResp)
		return
	}

	resp, _ := json.Marshal(openDoorResponse{Success: true})
	w.Write(resp)
	return
}

func main() {
	listen, err := net.Listen("tcp", ":15000")
	if err != nil {
		log.Panicln(err)
	}
	s := grpc.NewServer()
	doorservicepb.RegisterDoorServiceServer(s, &grpcServer{})

	reflection.Register(s)
	go func() {
		if err := s.Serve(listen); err != nil {
			log.Panicln(err)
		}
	}()
	http.HandleFunc("/api/v1/door/open", openDoorApi)
	http.ListenAndServe(":8000", nil)
}
