// Copyright 2026 The frp Authors
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

package features

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"

	"github.com/fatedier/frp/test/e2e/framework"
	"github.com/fatedier/frp/test/e2e/framework/consts"
	"github.com/fatedier/frp/test/e2e/pkg/relay"
	"github.com/fatedier/frp/test/e2e/pkg/request"
)

type adminAPI interface {
	Do(context.Context, string, string, []byte) (int, []byte, error)
}

var _ = ginkgo.Describe("[Feature: ControlReplacement]", func() {
	f := framework.NewDefaultFramework()

	for _, wireProtocol := range []string{"v1", "v2"} {
		for _, tcpMux := range []bool{true, false} {
			ginkgo.It(fmt.Sprintf("recovers a %s control through a half-open relay with tcpMux=%t", wireProtocol, tcpMux), func() {
				runHalfOpenControlReplacement(f, wireProtocol, tcpMux)
			})
		}
	}
})

func runHalfOpenControlReplacement(f *framework.Framework, wireProtocol string, tcpMux bool) {
	serverPort := f.AllocPort()
	dashboardPort := f.AllocPort()
	remotePort := f.AllocPort()
	heartbeatTimeout := int64(-1)
	if !tcpMux {
		heartbeatTimeout = 3
	}

	serverConfig := fmt.Sprintf(`
bindAddr = "127.0.0.1"
bindPort = %d
log.level = "trace"
transport.tcpMux = %t
transport.tcpMuxKeepaliveInterval = 30
transport.heartbeatTimeout = %d
webServer.addr = "127.0.0.1"
webServer.port = %d
webServer.sessionAuth.enabled = true
`, serverPort, tcpMux, heartbeatTimeout, dashboardPort)
	serverConfigPath := f.WriteTempFile("issue-5391-frps.toml", serverConfig)
	serverProcess, _, err := f.StartFrps("-c", serverConfigPath)
	framework.ExpectNoError(err)
	framework.ExpectNoError(framework.WaitForTCPReady(fmt.Sprintf("127.0.0.1:%d", serverPort), 5*time.Second))
	framework.ExpectNoError(framework.WaitForTCPReady(fmt.Sprintf("127.0.0.1:%d", dashboardPort), 5*time.Second))
	adminClient := f.APIClientForFrpc(dashboardPort)

	halfOpenRelay := relay.New(fmt.Sprintf("127.0.0.1:%d", serverPort))
	f.RunServer("", halfOpenRelay)

	heartbeatInterval := int64(-1)
	clientHeartbeatTimeout := int64(-1)
	if !tcpMux {
		heartbeatInterval = 1
		clientHeartbeatTimeout = 3
	}
	clientConfig := fmt.Sprintf(`
serverAddr = "127.0.0.1"
serverPort = %d
clientID = "issue-5391"
loginFailExit = false
log.level = "trace"
transport.wireProtocol = %q
transport.tcpMux = %t
transport.tcpMuxKeepaliveInterval = 1
transport.heartbeatInterval = %d
transport.heartbeatTimeout = %d
transport.tls.enable = false

[[proxies]]
name = "issue-5391-tcp"
type = "tcp"
localPort = %d
remotePort = %d
`, halfOpenRelay.BindPort(), wireProtocol, tcpMux, heartbeatInterval, clientHeartbeatTimeout,
		f.PortByName(framework.TCPEchoServerPort), remotePort)
	clientConfigPath := f.WriteTempFile("issue-5391-frpc.toml", clientConfig)
	clientProcess, _, err := f.StartFrpc("-c", clientConfigPath)
	framework.ExpectNoError(err)
	framework.ExpectNoError(halfOpenRelay.WaitForConnections(1, 5*time.Second))
	framework.ExpectNoError(clientProcess.WaitForOutput("login to server success", 1, 10*time.Second))
	framework.ExpectNoError(clientProcess.WaitForOutput("[issue-5391-tcp] start proxy success", 1, 10*time.Second))
	framework.ExpectNoError(waitForReplacementState(adminClient, remotePort, 10*time.Second))

	replacementCount := 1
	if tcpMux {
		replacementCount = 3
	}
	for i := 0; i < replacementCount; i++ {
		connectionIndex := 1
		if tcpMux {
			connectionIndex = i + 1
		}
		framework.ExpectNoError(halfOpenRelay.Blackhole(connectionIndex))
		if tcpMux {
			framework.ExpectNoError(halfOpenRelay.WaitForConnections(connectionIndex+1, 15*time.Second))
		}
		framework.ExpectNoError(clientProcess.WaitForOutput("login to server success", i+2, 15*time.Second))
		framework.ExpectNoError(clientProcess.WaitForOutput("[issue-5391-tcp] start proxy success", i+2, 15*time.Second))
		framework.ExpectNoError(waitForReplacementState(adminClient, remotePort, 10*time.Second))
	}

	_ = clientProcess.Stop()
	select {
	case <-clientProcess.Done():
	case <-time.After(5 * time.Second):
		framework.Failf("frpc did not exit")
	}
	framework.ExpectNoError(waitForReplacementShutdown(adminClient, 10*time.Second))
	framework.ExpectNoError(halfOpenRelay.Close())

	_ = serverProcess.Stop()
	select {
	case <-serverProcess.Done():
	case <-time.After(5 * time.Second):
		framework.Failf("frps did not exit")
	}
}

func waitForReplacementState(client adminAPI, remotePort int, timeout time.Duration) error {
	return waitForLifecycleCondition(timeout, func() error {
		if err := expectLifecycleCounts(client, 1, 1); err != nil {
			return err
		}
		clients, err := getOnlineLifecycleClients(client)
		if err != nil {
			return err
		}
		if len(clients) != 1 || clients[0].ClientID != "issue-5391" {
			return fmt.Errorf("expected one online client, got %+v", clients)
		}
		resp, err := request.New().
			TCP().
			Port(remotePort).
			Timeout(time.Second).
			Body([]byte(consts.TestString)).
			Do()
		if err != nil {
			return err
		}
		if string(resp.Content) != consts.TestString {
			return fmt.Errorf("unexpected proxy response %q", resp.Content)
		}
		return nil
	})
}

func waitForReplacementShutdown(client adminAPI, timeout time.Duration) error {
	return waitForLifecycleCondition(timeout, func() error {
		if err := expectLifecycleCounts(client, 0, 0); err != nil {
			return err
		}
		clients, err := getOnlineLifecycleClients(client)
		if err != nil {
			return err
		}
		if len(clients) != 0 {
			return fmt.Errorf("expected no online clients, got %+v", clients)
		}
		return nil
	})
}

type lifecycleClient struct {
	ClientID string `json:"clientID"`
}

func getOnlineLifecycleClients(client adminAPI) ([]lifecycleClient, error) {
	status, body, err := client.Do(context.Background(), "GET", "/api/v2/clients?status=online&page=1&pageSize=20", nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("GET online clients returned %d: %s", status, body)
	}
	var envelope struct {
		Data struct {
			Items []lifecycleClient `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	return envelope.Data.Items, nil
}

func expectLifecycleCounts(client adminAPI, onlineClients, proxies int) error {
	status, body, err := client.Do(context.Background(), "GET", "/api/v2/observability/summary", nil)
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("GET observability summary returned %d: %s", status, body)
	}
	var envelope struct {
		Data struct {
			OnlineClients int `json:"onlineClients"`
			Proxies       int `json:"proxies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}
	if envelope.Data.OnlineClients != onlineClients || envelope.Data.Proxies != proxies {
		return fmt.Errorf("lifecycle counts onlineClients=%d proxies=%d, want onlineClients=%d proxies=%d",
			envelope.Data.OnlineClients, envelope.Data.Proxies, onlineClients, proxies)
	}
	return nil
}

func waitForLifecycleCondition(timeout time.Duration, condition func() error) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		err := condition()
		if err == nil {
			return nil
		}
		lastErr = err
		select {
		case <-ticker.C:
		case <-timer.C:
			return fmt.Errorf("condition was not met: %w", lastErr)
		}
	}
}
