package consts

import (
	"fmt"
	"time"

	"github.com/fatedier/frp/test/e2e/pkg/port"
)

const (
	TestString = "frp is a fast reverse proxy to help you expose a local server behind a NAT or firewall to the internet."

	DefaultTimeout = 2 * time.Second
	AdminUsername  = "admin"
	AdminPassword  = "admin"
)

var (
	PortServerName  string
	PortClientAdmin string

	DefaultServerConfig = `
bindPort = {{ .%s }}
log.level = "trace"
webServer.sessionAuth.enabled = true
`

	DefaultClientConfig = `
serverAddr = "127.0.0.1"
serverPort = {{ .%s }}
loginFailExit = false
log.level = "trace"
webServer.sessionAuth.enabled = true
`
)

func init() {
	PortServerName = port.GenName("Server")
	PortClientAdmin = port.GenName("ClientAdmin")

	DefaultServerConfig = fmt.Sprintf(DefaultServerConfig, port.GenName("Server"))
	DefaultClientConfig = fmt.Sprintf(DefaultClientConfig, port.GenName("Server"))
}
