// Copyright 2019-present Open Networking Foundation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"fmt"
	"github.com/atomix/atomix-api/go/atomix/protocol"
	"github.com/atomix/atomix-go-framework/pkg/atomix/cluster"
	"github.com/atomix/atomix-go-framework/pkg/atomix/logging"
	"github.com/atomix/atomix-go-framework/pkg/atomix/storage/protocol/gossip"
	"github.com/atomix/atomix-go-framework/pkg/atomix/storage/protocol/gossip/counter"
	"github.com/atomix/atomix-go-framework/pkg/atomix/storage/protocol/gossip/map"
	"github.com/atomix/atomix-go-framework/pkg/atomix/storage/protocol/gossip/set"
	"github.com/atomix/atomix-go-framework/pkg/atomix/storage/protocol/gossip/value"
	"github.com/gogo/protobuf/jsonpb"
	"io/ioutil"
	"os"
	"os/signal"
)

func main() {
	logging.SetLevel(logging.DebugLevel)

	nodeID := os.Args[1]
	protocolConfig := parseProtocolConfig()

	cluster := cluster.NewCluster(cluster.NewNetwork(), protocolConfig, cluster.WithMemberID(nodeID))

	// Create an Atomix node
	node := gossip.NewNode(cluster)

	// Register primitives on the Atomix node
	counter.RegisterService(node)
	_map.RegisterService(node)
	set.RegisterService(node)
	value.RegisterService(node)

	counter.RegisterServer(node)
	_map.RegisterServer(node)
	set.RegisterServer(node)
	value.RegisterServer(node)

	// Start the node
	if err := node.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Wait for an interrupt signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

	// Stop the node after an interrupt
	if err := node.Stop(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func parseProtocolConfig() protocol.ProtocolConfig {
	configFile := os.Args[2]
	config := protocol.ProtocolConfig{}
	nodeBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := jsonpb.Unmarshal(bytes.NewReader(nodeBytes), &config); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return config
}
