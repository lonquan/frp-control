package basic

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/tidwall/gjson"

	"github.com/fatedier/frp/test/e2e/framework"
	"github.com/fatedier/frp/test/e2e/framework/consts"
)

var _ = ginkgo.Describe("[Feature: Annotations]", func() {
	f := framework.NewDefaultFramework()

	ginkgo.It("Set Proxy Annotations", func() {
		webPort := f.AllocPort()

		serverConf := consts.DefaultServerConfig + fmt.Sprintf(`
		webServer.port = %d
		`, webPort)

		p1Port := f.AllocPort()

		clientConf := consts.DefaultClientConfig + fmt.Sprintf(`
		[[proxies]]
		name = "p1"
		type = "tcp"
		localPort = {{ .%s }}
		remotePort = %d
		[proxies.annotations]
		"frp.e2e.test/foo" = "value1"
		"frp.e2e.test/bar" = "value2"
		`, framework.TCPEchoServerPort, p1Port)

		f.RunProcesses(serverConf, []string{clientConf})

		framework.NewRequestExpect(f).Port(p1Port).Ensure()

		// check annotations in frps through the authenticated v2 API
		status, content, err := f.APIClientForFrpc(webPort).Do(context.Background(), "GET", "/api/v2/proxies/p1", nil)
		framework.ExpectNoError(err)
		framework.ExpectEqual(status, 200)

		annotations := gjson.Get(string(content), "data.spec.tcp.annotations").Map()
		framework.ExpectEqual("value1", annotations["frp.e2e.test/foo"].String())
		framework.ExpectEqual("value2", annotations["frp.e2e.test/bar"].String())
	})
})
