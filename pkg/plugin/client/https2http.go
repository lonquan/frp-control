// Copyright 2019 fatedier, fatedier@gmail.com
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

//go:build !frps

package client

import (
	"net/http"
	"net/http/httputil"

	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/fatedier/frp/pkg/trafficcapture"
)

func init() {
	Register(v1.PluginHTTPS2HTTP, NewHTTPS2HTTPPlugin)
}

type HTTPS2HTTPPlugin struct {
	opts *v1.HTTPS2HTTPPluginOptions

	*httpBridgePlugin
}

func NewHTTPS2HTTPPlugin(pluginCtx PluginContext, options v1.ClientPluginOptions) (Plugin, error) {
	opts := options.(*v1.HTTPS2HTTPPluginOptions)

	p := &HTTPS2HTTPPlugin{
		opts: opts,
	}

	rp := newHTTPBridgeReverseProxy(
		func(r *httputil.ProxyRequest) {
			appendForwardedHeaders(r)
			req := r.Out
			rewriteHTTPPluginRequest(req, "http", p.opts.LocalAddr, p.opts.HostHeaderRewrite, p.opts.RequestHeaders)
		},
		nil,
	)
	handler := http.Handler(rp)
	if captures := trafficcapture.GetDefault(); captures != nil {
		handler = captures.WrapHTTPHandler(pluginCtx.Name, "https", "http", opts.LocalAddr, rp)
	}

	server, err := newHTTPSBridgePluginServer(handler, p.opts.CrtPath, p.opts.KeyPath, opts.EnableHTTP2, true)
	if err != nil {
		return nil, err
	}
	p.httpBridgePlugin = server

	return p, nil
}

func (p *HTTPS2HTTPPlugin) Name() string {
	return v1.PluginHTTPS2HTTP
}
