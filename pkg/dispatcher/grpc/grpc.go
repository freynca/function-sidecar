/*
 * Copyright 2017 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package grpc

import (
	"google.golang.org/grpc"

	"fmt"
	"log"
	"time"

	"github.com/projectriff/function-sidecar/pkg/dispatcher"
	"github.com/projectriff/function-sidecar/pkg/dispatcher/grpc/fntypes"
	"github.com/projectriff/function-sidecar/pkg/dispatcher/grpc/function"
	"golang.org/x/net/context"
)

type grpcDispatcher struct {
	stream function.StringFunction_CallClient
	input  chan dispatcher.Message
	output chan dispatcher.Message
}

func (this *grpcDispatcher) Input() chan<- dispatcher.Message {
	return this.input
}

func (this *grpcDispatcher) Output() <-chan dispatcher.Message {
	return this.output
}

func (this *grpcDispatcher) handleIncoming() {
	for {
		select {
		case in, open := <-this.input:
			if open {
				req := &fntypes.Request{Body: string(in.Payload)}
				err := this.stream.Send(req)
				if err != nil {
					log.Printf("Error sending message to function: %v", err)
				}
			} else {
				close(this.output)
				log.Print("Shutting down gRPC dispatcher")
				return
			}
		}
	}
}

func (this *grpcDispatcher) handleOutgoing() {
	for {
		reply, err := this.stream.Recv()
		if err != nil {
			log.Printf("Error receiving message from function: %v", err)
			continue
		}
		message := dispatcher.Message{Payload: []byte(reply.GetBody())}
		this.output <- message
	}
}

func NewGrpcDispatcher(port int) (dispatcher.Dispatcher, error) {
	ctx, _ := context.WithTimeout(context.Background(), 60*time.Second)
	conn, err := grpc.DialContext(ctx, fmt.Sprintf("localhost:%v", port), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, err
	}

	fnStream, err := function.NewStringFunctionClient(conn).Call(context.Background())
	if err != nil {
		return nil, err
	}

	result := &grpcDispatcher{fnStream, make(chan dispatcher.Message, 100), make(chan dispatcher.Message, 100)}
	go result.handleIncoming()
	go result.handleOutgoing()

	return result, nil
}
