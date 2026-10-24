// Copyright 2025 The frp Authors
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

package http

import (
	"cmp"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/fatedier/frp/client/configmgmt"
	"github.com/fatedier/frp/client/http/model"
	"github.com/fatedier/frp/client/proxy"
	"github.com/fatedier/frp/client/visitor"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	httppkg "github.com/fatedier/frp/pkg/util/http"
	"github.com/fatedier/frp/pkg/util/jsonx"
)

// Controller handles HTTP API requests for frpc.
type Controller struct {
	serverAddr         string
	manager            configmgmt.ConfigManager
	resolveCertificate func(id string, domains []string) (certPath, keyPath string, err error)
}

// ControllerParams contains parameters for creating an APIController.
type ControllerParams struct {
	ServerAddr         string
	Manager            configmgmt.ConfigManager
	ResolveCertificate func(id string, domains []string) (certPath, keyPath string, err error)
}

func NewController(params ControllerParams) *Controller {
	return &Controller{
		serverAddr:         params.ServerAddr,
		manager:            params.Manager,
		resolveCertificate: params.ResolveCertificate,
	}
}

func (c *Controller) toHTTPError(err error) error {
	if err == nil {
		return nil
	}

	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, configmgmt.ErrInvalidArgument):
		code = http.StatusBadRequest
	case errors.Is(err, configmgmt.ErrNotFound), errors.Is(err, configmgmt.ErrStoreDisabled):
		code = http.StatusNotFound
	case errors.Is(err, configmgmt.ErrConflict):
		code = http.StatusConflict
	}
	return httppkg.NewError(code, err.Error())
}

func (c *Controller) prepareCertificate(payload *model.ProxyDefinition) error {
	var (
		base          *v1.ProxyBaseConfig
		customDomains []string
	)
	switch proxyConfig := activeProxyBlock(payload).(type) {
	case *v1.TCPProxyConfig:
		base = proxyConfig.GetBaseConfig()
	case *v1.UDPProxyConfig:
		base = proxyConfig.GetBaseConfig()
	case *v1.HTTPProxyConfig:
		base = proxyConfig.GetBaseConfig()
		customDomains = proxyConfig.CustomDomains
	case *v1.HTTPSProxyConfig:
		base = proxyConfig.GetBaseConfig()
		customDomains = proxyConfig.CustomDomains
	case *v1.TCPMuxProxyConfig:
		base = proxyConfig.GetBaseConfig()
		customDomains = proxyConfig.CustomDomains
	case *v1.STCPProxyConfig:
		base = proxyConfig.GetBaseConfig()
	case *v1.SUDPProxyConfig:
		base = proxyConfig.GetBaseConfig()
	case *v1.XTCPProxyConfig:
		base = proxyConfig.GetBaseConfig()
	default:
		return errors.New("代理配置类型无效")
	}
	if base == nil {
		return errors.New("代理配置为空")
	}
	if payload.CertificateID == "" {
		delete(base.Metadatas, v1.CertificateIDMetadataKey)
		return nil
	}
	if c.resolveCertificate == nil {
		return errors.New("ACME 证书目录不可用")
	}
	certPath, keyPath, err := c.resolveCertificate(payload.CertificateID, customDomains)
	if err != nil {
		return err
	}
	switch options := base.Plugin.ClientPluginOptions.(type) {
	case *v1.HTTPS2HTTPPluginOptions:
		options.CrtPath = certPath
		options.KeyPath = keyPath
	case *v1.HTTPS2HTTPSPluginOptions:
		options.CrtPath = certPath
		options.KeyPath = keyPath
	case *v1.TLS2RawPluginOptions:
		options.CrtPath = certPath
		options.KeyPath = keyPath
	default:
		return errors.New("当前插件类型不支持 ACME 托管证书")
	}
	if base.Metadatas == nil {
		base.Metadatas = make(map[string]string)
	}
	base.Metadatas[v1.CertificateIDMetadataKey] = payload.CertificateID
	return nil
}

func activeProxyBlock(payload *model.ProxyDefinition) v1.ProxyConfigurer {
	switch {
	case payload.TCP != nil:
		return payload.TCP
	case payload.UDP != nil:
		return payload.UDP
	case payload.HTTP != nil:
		return payload.HTTP
	case payload.HTTPS != nil:
		return payload.HTTPS
	case payload.TCPMux != nil:
		return payload.TCPMux
	case payload.STCP != nil:
		return payload.STCP
	case payload.SUDP != nil:
		return payload.SUDP
	case payload.XTCP != nil:
		return payload.XTCP
	default:
		return nil
	}
}

// Reload handles GET /api/reload
func (c *Controller) Reload(ctx *httppkg.Context) (any, error) {
	strictConfigMode := false
	strictStr := ctx.Query("strictConfig")
	if strictStr != "" {
		strictConfigMode, _ = strconv.ParseBool(strictStr)
	}

	if err := c.manager.ReloadFromFile(strictConfigMode); err != nil {
		return nil, c.toHTTPError(err)
	}
	return nil, nil
}

// Stop handles POST /api/stop
func (c *Controller) Stop(ctx *httppkg.Context) (any, error) {
	go c.manager.GracefulClose(100 * time.Millisecond)
	return nil, nil
}

// Status handles GET /api/status
func (c *Controller) Status(ctx *httppkg.Context) (any, error) {
	res := make(model.StatusResp)
	ps := c.manager.GetProxyStatus()
	if ps == nil {
		return res, nil
	}

	for _, status := range ps {
		res[status.Type] = append(res[status.Type], c.buildProxyStatusResp(status))
	}

	for _, arrs := range res {
		if len(arrs) <= 1 {
			continue
		}
		slices.SortFunc(arrs, func(a, b model.ProxyStatusResp) int {
			return cmp.Compare(a.Name, b.Name)
		})
	}
	return res, nil
}

// ListProxyConfigs handles GET /api/config/proxies.
func (c *Controller) ListProxyConfigs(ctx *httppkg.Context) (any, error) {
	entries, err := c.manager.ListProxyConfigs()
	if err != nil {
		return nil, c.toHTTPError(err)
	}

	resp := model.ProxyConfigListResp{
		Proxies: make([]model.ProxyConfigSummary, 0, len(entries)),
	}
	for _, entry := range entries {
		resp.Proxies = append(resp.Proxies, c.buildProxyConfigSummary(entry))
	}
	slices.SortFunc(resp.Proxies, func(a, b model.ProxyConfigSummary) int {
		if byName := cmp.Compare(a.Name, b.Name); byName != 0 {
			return byName
		}
		return cmp.Compare(a.Source, b.Source)
	})
	return resp, nil
}

// ListVisitorConfigs handles GET /api/config/visitors.
func (c *Controller) ListVisitorConfigs(ctx *httppkg.Context) (any, error) {
	entries, err := c.manager.ListVisitorConfigs()
	if err != nil {
		return nil, c.toHTTPError(err)
	}

	resp := model.VisitorConfigListResp{
		Visitors: make([]model.VisitorConfigSummary, 0, len(entries)),
	}
	for _, entry := range entries {
		resp.Visitors = append(resp.Visitors, buildVisitorConfigSummary(entry))
	}
	slices.SortFunc(resp.Visitors, func(a, b model.VisitorConfigSummary) int {
		if byName := cmp.Compare(a.Name, b.Name); byName != 0 {
			return byName
		}
		return cmp.Compare(a.Source, b.Source)
	})
	return resp, nil
}

// ListVisitorStatus handles GET /api/visitors/status.
func (c *Controller) ListVisitorStatus(ctx *httppkg.Context) (any, error) {
	statuses := c.manager.GetVisitorStatus()
	resp := model.VisitorStatusListResp{
		Visitors: make([]model.VisitorStatusResp, 0, len(statuses)),
	}
	for _, status := range statuses {
		resp.Visitors = append(resp.Visitors, buildVisitorStatusResp(status))
	}
	slices.SortFunc(resp.Visitors, func(a, b model.VisitorStatusResp) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return resp, nil
}

// GetConfig handles GET /api/config
func (c *Controller) GetConfig(ctx *httppkg.Context) (any, error) {
	content, err := c.manager.ReadConfigFile()
	if err != nil {
		return nil, c.toHTTPError(err)
	}
	return content, nil
}

// PutConfig handles PUT /api/config
func (c *Controller) PutConfig(ctx *httppkg.Context) (any, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, fmt.Sprintf("read request body error: %v", err))
	}

	if len(body) == 0 {
		return nil, httppkg.NewError(http.StatusBadRequest, "body can't be empty")
	}

	if err := c.manager.WriteConfigFile(body); err != nil {
		return nil, c.toHTTPError(err)
	}
	return nil, nil
}

func (c *Controller) buildProxyStatusResp(status *proxy.WorkingStatus) model.ProxyStatusResp {
	psr := model.ProxyStatusResp{
		Name:   status.Name,
		Type:   status.Type,
		Status: status.Phase,
		Err:    status.Err,
	}
	baseCfg := status.Cfg.GetBaseConfig()
	psr.CertificateID = baseCfg.Metadatas[v1.CertificateIDMetadataKey]
	psr.LocalAddr = baseCfg.LocalServiceAddr()
	psr.Plugin = baseCfg.Plugin.Type

	if status.Err == "" || status.Phase == proxy.ProxyPhaseRunning {
		psr.RemoteAddr = status.RemoteAddr
		if slices.Contains([]string{"tcp", "udp"}, status.Type) {
			psr.RemoteAddr = c.serverAddr + psr.RemoteAddr
		}
	}

	if c.manager.IsStoreProxyEnabled(status.Name) {
		psr.Source = model.SourceStore
	}
	return psr
}

func (c *Controller) buildProxyConfigSummary(entry configmgmt.ProxyConfigEntry) model.ProxyConfigSummary {
	base := entry.Config.GetBaseConfig()
	summary := model.ProxyConfigSummary{
		Name:          base.Name,
		Type:          base.Type,
		Source:        entry.Source,
		Enabled:       base.Enabled == nil || *base.Enabled,
		Plugin:        base.Plugin.Type,
		CertificateID: base.Metadatas[v1.CertificateIDMetadataKey],
	}
	summary.LocalAddr = base.LocalServiceAddr()

	switch cfg := entry.Config.(type) {
	case *v1.TCPProxyConfig:
		if cfg.RemotePort != 0 {
			summary.RemoteAddr = net.JoinHostPort(c.serverAddr, strconv.Itoa(cfg.RemotePort))
		}
	case *v1.UDPProxyConfig:
		if cfg.RemotePort != 0 {
			summary.RemoteAddr = net.JoinHostPort(c.serverAddr, strconv.Itoa(cfg.RemotePort))
		}
	case *v1.HTTPProxyConfig:
		summary.RemoteAddr = remoteDomain(cfg.CustomDomains, cfg.SubDomain)
	case *v1.HTTPSProxyConfig:
		summary.RemoteAddr = remoteDomain(cfg.CustomDomains, cfg.SubDomain)
	case *v1.TCPMuxProxyConfig:
		summary.RemoteAddr = remoteDomain(cfg.CustomDomains, cfg.SubDomain)
	}
	return summary
}

func buildVisitorConfigSummary(entry configmgmt.VisitorConfigEntry) model.VisitorConfigSummary {
	base := entry.Config.GetBaseConfig()
	bindAddr := base.BindAddr
	if bindAddr == "" {
		bindAddr = "127.0.0.1"
	}
	summary := model.VisitorConfigSummary{
		Name:             base.Name,
		Type:             base.Type,
		Source:           entry.Source,
		Enabled:          base.Enabled == nil || *base.Enabled,
		SecretConfigured: base.SecretKey != "",
		ServerUser:       base.ServerUser,
		ServerName:       base.ServerName,
		BindAddr:         bindAddr,
		BindPort:         base.BindPort,
		Plugin:           base.Plugin.Type,
		UseEncryption:    base.Transport.UseEncryption,
		UseCompression:   base.Transport.UseCompression,
	}
	if xtcp, ok := entry.Config.(*v1.XTCPVisitorConfig); ok {
		summary.XTCPProtocol = xtcp.Protocol
		summary.KeepTunnelOpen = xtcp.KeepTunnelOpen
		summary.FallbackTo = xtcp.FallbackTo
	}
	return summary
}

func buildVisitorStatusResp(status *visitor.WorkingStatus) model.VisitorStatusResp {
	bindAddr := status.BindAddr
	if bindAddr == "" {
		bindAddr = "127.0.0.1"
	}
	return model.VisitorStatusResp{
		Name:       status.Name,
		Type:       status.Type,
		Status:     status.Status,
		Err:        status.Err,
		BindAddr:   bindAddr,
		BindPort:   status.BindPort,
		ServerUser: status.ServerUser,
		ServerName: status.ServerName,
		Plugin:     status.Plugin,
	}
}

func remoteDomain(customDomains []string, subdomain string) string {
	if len(customDomains) > 0 {
		return strings.Join(customDomains, ", ")
	}
	return subdomain
}

// GetProxyConfig handles GET /api/proxy/{name}/config
func (c *Controller) GetProxyConfig(ctx *httppkg.Context) (any, error) {
	name := ctx.Param("name")
	if name == "" {
		return nil, httppkg.NewError(http.StatusBadRequest, "proxy name is required")
	}

	cfg, ok := c.manager.GetProxyConfig(name)
	if !ok {
		return nil, httppkg.NewError(http.StatusNotFound, fmt.Sprintf("proxy %q not found", name))
	}

	payload, err := model.ProxyDefinitionFromConfigurer(cfg)
	if err != nil {
		return nil, httppkg.NewError(http.StatusInternalServerError, err.Error())
	}
	return payload, nil
}

// GetVisitorConfig handles GET /api/visitor/{name}/config
func (c *Controller) GetVisitorConfig(ctx *httppkg.Context) (any, error) {
	name := ctx.Param("name")
	if name == "" {
		return nil, httppkg.NewError(http.StatusBadRequest, "visitor name is required")
	}

	cfg, ok := c.manager.GetVisitorConfig(name)
	if !ok {
		return nil, httppkg.NewError(http.StatusNotFound, fmt.Sprintf("visitor %q not found", name))
	}

	payload, err := model.VisitorDefinitionFromConfigurer(cfg)
	if err != nil {
		return nil, httppkg.NewError(http.StatusInternalServerError, err.Error())
	}
	return payload, nil
}

func (c *Controller) ListStoreProxies(ctx *httppkg.Context) (any, error) {
	proxies, err := c.manager.ListStoreProxies()
	if err != nil {
		return nil, c.toHTTPError(err)
	}

	resp := model.ProxyListResp{Proxies: make([]model.ProxyDefinition, 0, len(proxies))}
	for _, p := range proxies {
		payload, err := model.ProxyDefinitionFromConfigurer(p)
		if err != nil {
			return nil, httppkg.NewError(http.StatusInternalServerError, err.Error())
		}
		resp.Proxies = append(resp.Proxies, payload)
	}
	slices.SortFunc(resp.Proxies, func(a, b model.ProxyDefinition) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return resp, nil
}

func (c *Controller) GetStoreProxy(ctx *httppkg.Context) (any, error) {
	name := ctx.Param("name")
	if name == "" {
		return nil, httppkg.NewError(http.StatusBadRequest, "proxy name is required")
	}

	p, err := c.manager.GetStoreProxy(name)
	if err != nil {
		return nil, c.toHTTPError(err)
	}

	payload, err := model.ProxyDefinitionFromConfigurer(p)
	if err != nil {
		return nil, httppkg.NewError(http.StatusInternalServerError, err.Error())
	}

	return payload, nil
}

func (c *Controller) CreateStoreProxy(ctx *httppkg.Context) (any, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, fmt.Sprintf("read body error: %v", err))
	}

	var payload model.ProxyDefinition
	if err := jsonx.Unmarshal(body, &payload); err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, fmt.Sprintf("parse JSON error: %v", err))
	}

	if err := payload.Validate("", false); err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, err.Error())
	}
	if err := c.prepareCertificate(&payload); err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, err.Error())
	}
	cfg, err := payload.ToConfigurer()
	if err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, err.Error())
	}
	created, err := c.manager.CreateStoreProxy(cfg)
	if err != nil {
		return nil, c.toHTTPError(err)
	}

	resp, err := model.ProxyDefinitionFromConfigurer(created)
	if err != nil {
		return nil, httppkg.NewError(http.StatusInternalServerError, err.Error())
	}
	return resp, nil
}

func (c *Controller) UpdateStoreProxy(ctx *httppkg.Context) (any, error) {
	name := ctx.Param("name")
	if name == "" {
		return nil, httppkg.NewError(http.StatusBadRequest, "proxy name is required")
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, fmt.Sprintf("read body error: %v", err))
	}

	var payload model.ProxyDefinition
	if err := jsonx.Unmarshal(body, &payload); err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, fmt.Sprintf("parse JSON error: %v", err))
	}

	if err := payload.Validate(name, true); err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, err.Error())
	}
	if err := c.prepareCertificate(&payload); err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, err.Error())
	}
	cfg, err := payload.ToConfigurer()
	if err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, err.Error())
	}
	updated, err := c.manager.UpdateStoreProxy(name, cfg)
	if err != nil {
		return nil, c.toHTTPError(err)
	}

	resp, err := model.ProxyDefinitionFromConfigurer(updated)
	if err != nil {
		return nil, httppkg.NewError(http.StatusInternalServerError, err.Error())
	}
	return resp, nil
}

func (c *Controller) DeleteStoreProxy(ctx *httppkg.Context) (any, error) {
	name := ctx.Param("name")
	if name == "" {
		return nil, httppkg.NewError(http.StatusBadRequest, "proxy name is required")
	}

	if err := c.manager.DeleteStoreProxy(name); err != nil {
		return nil, c.toHTTPError(err)
	}
	return nil, nil
}

func (c *Controller) ListStoreVisitors(ctx *httppkg.Context) (any, error) {
	visitors, err := c.manager.ListStoreVisitors()
	if err != nil {
		return nil, c.toHTTPError(err)
	}

	resp := model.VisitorListResp{Visitors: make([]model.VisitorDefinition, 0, len(visitors))}
	for _, v := range visitors {
		payload, err := model.VisitorDefinitionFromConfigurer(v)
		if err != nil {
			return nil, httppkg.NewError(http.StatusInternalServerError, err.Error())
		}
		resp.Visitors = append(resp.Visitors, payload)
	}
	slices.SortFunc(resp.Visitors, func(a, b model.VisitorDefinition) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return resp, nil
}

func (c *Controller) GetStoreVisitor(ctx *httppkg.Context) (any, error) {
	name := ctx.Param("name")
	if name == "" {
		return nil, httppkg.NewError(http.StatusBadRequest, "visitor name is required")
	}

	v, err := c.manager.GetStoreVisitor(name)
	if err != nil {
		return nil, c.toHTTPError(err)
	}

	payload, err := model.VisitorDefinitionFromConfigurer(v)
	if err != nil {
		return nil, httppkg.NewError(http.StatusInternalServerError, err.Error())
	}

	return payload, nil
}

func (c *Controller) CreateStoreVisitor(ctx *httppkg.Context) (any, error) {
	body, err := ctx.Body()
	if err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, fmt.Sprintf("read body error: %v", err))
	}

	var payload model.VisitorDefinition
	if err := jsonx.Unmarshal(body, &payload); err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, fmt.Sprintf("parse JSON error: %v", err))
	}

	if err := payload.Validate("", false); err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, err.Error())
	}
	cfg, err := payload.ToConfigurer()
	if err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, err.Error())
	}
	created, err := c.manager.CreateStoreVisitor(cfg)
	if err != nil {
		return nil, c.toHTTPError(err)
	}

	resp, err := model.VisitorDefinitionFromConfigurer(created)
	if err != nil {
		return nil, httppkg.NewError(http.StatusInternalServerError, err.Error())
	}
	return resp, nil
}

func (c *Controller) UpdateStoreVisitor(ctx *httppkg.Context) (any, error) {
	name := ctx.Param("name")
	if name == "" {
		return nil, httppkg.NewError(http.StatusBadRequest, "visitor name is required")
	}

	body, err := ctx.Body()
	if err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, fmt.Sprintf("read body error: %v", err))
	}

	var payload model.VisitorDefinition
	if err := jsonx.Unmarshal(body, &payload); err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, fmt.Sprintf("parse JSON error: %v", err))
	}

	if err := payload.Validate(name, true); err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, err.Error())
	}
	cfg, err := payload.ToConfigurer()
	if err != nil {
		return nil, httppkg.NewError(http.StatusBadRequest, err.Error())
	}
	updated, err := c.manager.UpdateStoreVisitor(name, cfg)
	if err != nil {
		return nil, c.toHTTPError(err)
	}

	resp, err := model.VisitorDefinitionFromConfigurer(updated)
	if err != nil {
		return nil, httppkg.NewError(http.StatusInternalServerError, err.Error())
	}
	return resp, nil
}

func (c *Controller) DeleteStoreVisitor(ctx *httppkg.Context) (any, error) {
	name := ctx.Param("name")
	if name == "" {
		return nil, httppkg.NewError(http.StatusBadRequest, "visitor name is required")
	}

	if err := c.manager.DeleteStoreVisitor(name); err != nil {
		return nil, c.toHTTPError(err)
	}
	return nil, nil
}
