package main

import (
	"context"
	"log"
	"time"

	doorservicepb "github.com/0xc0d3d00d/openify/go/proto/doorservice"
	"google.golang.org/grpc"
)

const (
	address234 = "127.0.0.1:15000"
)

type DoorClient struct {
	conn   *grpc.ClientConn
	client doorservicepb.DoorServiceClient
	serial int64
}

func New(conn *grpc.ClientConn, serial int64) *DoorClient {
	client := doorservicepb.NewDoorServiceClient(conn)

	return &DoorClient{
		conn:   conn,
		client: client,
		serial: serial,
	}
}

func (c *DoorClient) UpdateState(state doorservicepb.DoorState) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.client.UpdateState(ctx, &doorservicepb.UpdateStateRequest{State: state, Serial: c.serial})
	if err != nil {
		return err
	}
	return nil
}

type OpenCommandHandler func()

func (c *DoorClient) RunAccessStreamThread(handler OpenCommandHandler) error {
	ctx := context.Background()

	stream, err := c.client.AccessStream(ctx, &doorservicepb.AccessStreamRequest{Serial: c.serial})
	if err != nil {
		return err
	}
	//	go func() {
	for {
		resp, err := stream.Recv()
		if err != nil {
			return err
		}
		if resp.OpenDoor == true {
			handler()
		}
	}
	//	}()
	return nil
}

func PrintHandler() {
	log.Println("Door Opened")
}

func main() {

	conn, err := grpc.Dial(address234, grpc.WithInsecure())
	if err != nil {
		log.Panicln(err)
	}
	defer conn.Close()

	client := New(conn, 876543)
	err = client.RunAccessStreamThread(PrintHandler)
	log.Println(err)
}
