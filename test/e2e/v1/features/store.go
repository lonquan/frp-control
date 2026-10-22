package features

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"

	"github.com/fatedier/frp/test/e2e/framework"
	"github.com/fatedier/frp/test/e2e/framework/consts"
	"github.com/fatedier/frp/test/e2e/pkg/request"
)

var _ = ginkgo.Describe("[Feature: Store]", func() {
	f := framework.NewDefaultFramework()

	ginkgo.Describe("Store API", func() {
		ginkgo.It("create proxy via API and verify connection", func() {
			adminPort := f.AllocPort()
			remotePort := f.AllocPort()

			serverConf := consts.DefaultServerConfig
			clientConf := consts.DefaultClientConfig + fmt.Sprintf(`
			webServer.addr = "127.0.0.1"
			webServer.port = %d

			[store]
			path = "%s/store.json"
			`, adminPort, f.TempDirectory)

			f.RunProcesses(serverConf, []string{clientConf})
			framework.ExpectNoError(framework.WaitForTCPReady(fmt.Sprintf("127.0.0.1:%d", adminPort), 5*time.Second))

			proxyConfig := map[string]any{
				"name": "test-tcp",
				"type": "tcp",
				"tcp": map[string]any{
					"localIP":    "127.0.0.1",
					"localPort":  f.PortByName(framework.TCPEchoServerPort),
					"remotePort": remotePort,
				},
			}
			proxyBody, _ := json.Marshal(proxyConfig)

			resp := storeAPICall(f, adminPort, "POST", "/api/store/proxies", proxyBody)
			framework.ExpectEqual(resp.Code, 200)

			framework.ExpectNoError(framework.WaitForTCPReady(fmt.Sprintf("127.0.0.1:%d", remotePort), 5*time.Second))

			framework.NewRequestExpect(f).Port(remotePort).Ensure()
		})

		ginkgo.It("update proxy via API", func() {
			adminPort := f.AllocPort()
			remotePort1 := f.AllocPort()
			remotePort2 := f.AllocPort()

			serverConf := consts.DefaultServerConfig
			clientConf := consts.DefaultClientConfig + fmt.Sprintf(`
			webServer.addr = "127.0.0.1"
			webServer.port = %d

			[store]
			path = "%s/store.json"
			`, adminPort, f.TempDirectory)

			f.RunProcesses(serverConf, []string{clientConf})
			framework.ExpectNoError(framework.WaitForTCPReady(fmt.Sprintf("127.0.0.1:%d", adminPort), 5*time.Second))

			proxyConfig := map[string]any{
				"name": "test-tcp",
				"type": "tcp",
				"tcp": map[string]any{
					"localIP":    "127.0.0.1",
					"localPort":  f.PortByName(framework.TCPEchoServerPort),
					"remotePort": remotePort1,
				},
			}
			proxyBody, _ := json.Marshal(proxyConfig)

			resp := storeAPICall(f, adminPort, "POST", "/api/store/proxies", proxyBody)
			framework.ExpectEqual(resp.Code, 200)

			framework.ExpectNoError(framework.WaitForTCPReady(fmt.Sprintf("127.0.0.1:%d", remotePort1), 5*time.Second))
			framework.NewRequestExpect(f).Port(remotePort1).Ensure()

			proxyConfig["tcp"].(map[string]any)["remotePort"] = remotePort2
			proxyBody, _ = json.Marshal(proxyConfig)

			resp = storeAPICall(f, adminPort, "PUT", "/api/store/proxies/test-tcp", proxyBody)
			framework.ExpectEqual(resp.Code, 200)

			framework.ExpectNoError(framework.WaitForTCPReady(fmt.Sprintf("127.0.0.1:%d", remotePort2), 5*time.Second))
			framework.ExpectNoError(framework.WaitForTCPUnreachable(fmt.Sprintf("127.0.0.1:%d", remotePort1), 100*time.Millisecond, 5*time.Second))
			framework.NewRequestExpect(f).Port(remotePort2).Ensure()
			framework.NewRequestExpect(f).Port(remotePort1).ExpectError(true).Ensure()
		})

		ginkgo.It("delete proxy via API", func() {
			adminPort := f.AllocPort()
			remotePort := f.AllocPort()

			serverConf := consts.DefaultServerConfig
			clientConf := consts.DefaultClientConfig + fmt.Sprintf(`
			webServer.addr = "127.0.0.1"
			webServer.port = %d

			[store]
			path = "%s/store.json"
			`, adminPort, f.TempDirectory)

			f.RunProcesses(serverConf, []string{clientConf})
			framework.ExpectNoError(framework.WaitForTCPReady(fmt.Sprintf("127.0.0.1:%d", adminPort), 5*time.Second))

			proxyConfig := map[string]any{
				"name": "test-tcp",
				"type": "tcp",
				"tcp": map[string]any{
					"localIP":    "127.0.0.1",
					"localPort":  f.PortByName(framework.TCPEchoServerPort),
					"remotePort": remotePort,
				},
			}
			proxyBody, _ := json.Marshal(proxyConfig)

			resp := storeAPICall(f, adminPort, "POST", "/api/store/proxies", proxyBody)
			framework.ExpectEqual(resp.Code, 200)

			framework.ExpectNoError(framework.WaitForTCPReady(fmt.Sprintf("127.0.0.1:%d", remotePort), 5*time.Second))
			framework.NewRequestExpect(f).Port(remotePort).Ensure()

			resp = storeAPICall(f, adminPort, "DELETE", "/api/store/proxies/test-tcp", nil)
			framework.ExpectEqual(resp.Code, 200)

			framework.ExpectNoError(framework.WaitForTCPUnreachable(fmt.Sprintf("127.0.0.1:%d", remotePort), 100*time.Millisecond, 5*time.Second))
			framework.NewRequestExpect(f).Port(remotePort).ExpectError(true).Ensure()
		})

		ginkgo.It("list and get proxy via API", func() {
			adminPort := f.AllocPort()
			remotePort := f.AllocPort()

			serverConf := consts.DefaultServerConfig
			clientConf := consts.DefaultClientConfig + fmt.Sprintf(`
			webServer.addr = "127.0.0.1"
			webServer.port = %d

			[store]
			path = "%s/store.json"
			`, adminPort, f.TempDirectory)

			f.RunProcesses(serverConf, []string{clientConf})
			framework.ExpectNoError(framework.WaitForTCPReady(fmt.Sprintf("127.0.0.1:%d", adminPort), 5*time.Second))

			proxyConfig := map[string]any{
				"name": "test-tcp",
				"type": "tcp",
				"tcp": map[string]any{
					"localIP":    "127.0.0.1",
					"localPort":  f.PortByName(framework.TCPEchoServerPort),
					"remotePort": remotePort,
				},
			}
			proxyBody, _ := json.Marshal(proxyConfig)

			resp := storeAPICall(f, adminPort, "POST", "/api/store/proxies", proxyBody)
			framework.ExpectEqual(resp.Code, 200)

			resp = storeAPICall(f, adminPort, "GET", "/api/store/proxies", nil)
			framework.ExpectEqual(resp.Code, 200)
			framework.ExpectTrue(strings.Contains(string(resp.Content), "test-tcp"))

			resp = storeAPICall(f, adminPort, "GET", "/api/store/proxies/test-tcp", nil)
			framework.ExpectEqual(resp.Code, 200)
			framework.ExpectTrue(strings.Contains(string(resp.Content), "test-tcp"))

			resp = storeAPICall(f, adminPort, "GET", "/api/store/proxies/nonexistent", nil)
			framework.ExpectEqual(resp.Code, 404)
		})

		ginkgo.It("store disabled returns 404", func() {
			adminPort := f.AllocPort()

			serverConf := consts.DefaultServerConfig
			clientConf := consts.DefaultClientConfig + fmt.Sprintf(`
			webServer.addr = "127.0.0.1"
			webServer.port = %d
			`, adminPort)

			f.RunProcesses(serverConf, []string{clientConf})
			framework.ExpectNoError(framework.WaitForTCPReady(fmt.Sprintf("127.0.0.1:%d", adminPort), 5*time.Second))

			resp := storeAPICall(f, adminPort, "GET", "/api/store/proxies", nil)
			framework.ExpectEqual(resp.Code, 404)
		})

		ginkgo.It("rejects mismatched type block", func() {
			adminPort := f.AllocPort()

			serverConf := consts.DefaultServerConfig
			clientConf := consts.DefaultClientConfig + fmt.Sprintf(`
			webServer.addr = "127.0.0.1"
			webServer.port = %d

			[store]
			path = "%s/store.json"
			`, adminPort, f.TempDirectory)

			f.RunProcesses(serverConf, []string{clientConf})
			framework.ExpectNoError(framework.WaitForTCPReady(fmt.Sprintf("127.0.0.1:%d", adminPort), 5*time.Second))

			invalidBody, _ := json.Marshal(map[string]any{
				"name": "bad-proxy",
				"type": "tcp",
				"udp": map[string]any{
					"localPort": 1234,
				},
			})

			resp := storeAPICall(f, adminPort, "POST", "/api/store/proxies", invalidBody)
			framework.ExpectEqual(resp.Code, 400)
		})

		ginkgo.It("rejects path/body name mismatch on update", func() {
			adminPort := f.AllocPort()
			remotePort := f.AllocPort()

			serverConf := consts.DefaultServerConfig
			clientConf := consts.DefaultClientConfig + fmt.Sprintf(`
			webServer.addr = "127.0.0.1"
			webServer.port = %d

			[store]
			path = "%s/store.json"
			`, adminPort, f.TempDirectory)

			f.RunProcesses(serverConf, []string{clientConf})
			framework.ExpectNoError(framework.WaitForTCPReady(fmt.Sprintf("127.0.0.1:%d", adminPort), 5*time.Second))

			createBody, _ := json.Marshal(map[string]any{
				"name": "proxy-a",
				"type": "tcp",
				"tcp": map[string]any{
					"localIP":    "127.0.0.1",
					"localPort":  f.PortByName(framework.TCPEchoServerPort),
					"remotePort": remotePort,
				},
			})

			resp := storeAPICall(f, adminPort, "POST", "/api/store/proxies", createBody)
			framework.ExpectEqual(resp.Code, 200)

			updateBody, _ := json.Marshal(map[string]any{
				"name": "proxy-b",
				"type": "tcp",
				"tcp": map[string]any{
					"localIP":    "127.0.0.1",
					"localPort":  f.PortByName(framework.TCPEchoServerPort),
					"remotePort": remotePort,
				},
			})

			resp = storeAPICall(f, adminPort, "PUT", "/api/store/proxies/proxy-a", updateBody)
			framework.ExpectEqual(resp.Code, 400)
		})
	})
})

func storeAPICall(f *framework.Framework, port int, method, path string, body []byte) *request.Response {
	status, content, err := f.APIClientForFrpc(port).Do(context.Background(), method, path, body)
	framework.ExpectNoError(err)
	return &request.Response{Code: status, Content: content}
}
