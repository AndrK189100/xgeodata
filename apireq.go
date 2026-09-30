package main

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ReloadRequest asks Xray to reload geodata via its gRPC API.
// An empty host means the API is not configured, so nothing is done.
func ReloadRequest(host string) error {

	if host == "" {
		return nil
	}

	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return conn.Invoke(ctx, "/xray.app.router.command.RoutingService/ReloadGeoData", &emptypb.Empty{}, &emptypb.Empty{})
}