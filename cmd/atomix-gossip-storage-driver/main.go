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
	"fmt"
	"github.com/atomix/atomix-go-framework/pkg/atomix/cluster"
	"github.com/atomix/atomix-go-framework/pkg/atomix/driver"
	"github.com/atomix/atomix-go-framework/pkg/atomix/driver/env"
	"github.com/atomix/atomix-go-framework/pkg/atomix/driver/proxy"
	"github.com/atomix/atomix-go-framework/pkg/atomix/driver/proxy/gossip"
	"github.com/atomix/atomix-go-framework/pkg/atomix/driver/proxy/gossip/counter"
	_map "github.com/atomix/atomix-go-framework/pkg/atomix/driver/proxy/gossip/map"
	"github.com/atomix/atomix-go-framework/pkg/atomix/driver/proxy/gossip/set"
	"github.com/atomix/atomix-go-framework/pkg/atomix/driver/proxy/gossip/value"
	"github.com/atomix/atomix-go-framework/pkg/atomix/logging"
	"os"
	"os/signal"
	"strconv"
	"strings"
)

func main() {
	logging.SetLevel(logging.DebugLevel)

	address := os.Args[1]
	parts := strings.Split(address, ":")
	host, portNum := parts[0], parts[1]
	port, err := strconv.Atoi(portNum)
	if err != nil {
		panic(err)
	}

	provider := func(c cluster.Cluster, env env.DriverEnv) proxy.Protocol {
		p := gossip.NewProtocol(c, env)
		counter.Register(p)
		_map.Register(p)
		set.Register(p)
		value.Register(p)
		return p
	}

	// Create a gossip driver
	d := driver.NewDriver(
		provider,
		driver.WithDriverID("gossip"),
		driver.WithHost(host),
		driver.WithPort(port))

	// Start the node
	if err := d.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Wait for an interrupt signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

	// Stop the node after an interrupt
	if err := d.Stop(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
