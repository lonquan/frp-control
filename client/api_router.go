// Copyright 2017 fatedier, fatedier@gmail.com
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

package client

import (
	"net/http"

	adminapi "github.com/fatedier/frp/client/http"
	"github.com/fatedier/frp/client/proxy"
	"github.com/fatedier/frp/client/visitor"
	httppkg "github.com/fatedier/frp/pkg/util/http"
	netpkg "github.com/fatedier/frp/pkg/util/net"
)

func (svr *Service) registerRouteHandlers(helper *httppkg.RouterRegisterHelper) {
	apiController := newAPIController(svr)

	// Healthz endpoint without auth
	helper.Router.HandleFunc("/healthz", healthz)
	if svr.adminRuntime != nil {
		svr.adminRuntime.RegisterPublic(helper.Router)
	}

	// Management APIs are only available through the session-authenticated
	// admin runtime. There is deliberately no Basic Auth fallback.
	if svr.adminRuntime != nil {
		subRouter := helper.Router.NewRoute().Subrouter()
		subRouter.Use(svr.adminRuntime.Wrap)
		svr.adminRuntime.RegisterPrivate(subRouter)
		subRouter.Use(httppkg.NewRequestLogger)
		subRouter.HandleFunc("/api/reload", httppkg.MakeHTTPHandlerFunc(apiController.Reload)).Methods(http.MethodGet)
		subRouter.HandleFunc("/api/stop", httppkg.MakeHTTPHandlerFunc(apiController.Stop)).Methods(http.MethodPost)
		subRouter.HandleFunc("/api/status", httppkg.MakeHTTPHandlerFunc(apiController.Status)).Methods(http.MethodGet)
		subRouter.HandleFunc("/api/visitors/status", httppkg.MakeHTTPHandlerFunc(apiController.ListVisitorStatus)).Methods(http.MethodGet)
		subRouter.HandleFunc("/api/config", httppkg.MakeHTTPHandlerFunc(apiController.GetConfig)).Methods(http.MethodGet)
		subRouter.HandleFunc("/api/config", httppkg.MakeHTTPHandlerFunc(apiController.PutConfig)).Methods(http.MethodPut)
		subRouter.HandleFunc("/api/config/proxies", httppkg.MakeHTTPHandlerFunc(apiController.ListProxyConfigs)).Methods(http.MethodGet)
		subRouter.HandleFunc("/api/config/visitors", httppkg.MakeHTTPHandlerFunc(apiController.ListVisitorConfigs)).Methods(http.MethodGet)
		subRouter.HandleFunc("/api/proxy/{name}/config", httppkg.MakeHTTPHandlerFunc(apiController.GetProxyConfig)).Methods(http.MethodGet)
		subRouter.HandleFunc("/api/visitor/{name}/config", httppkg.MakeHTTPHandlerFunc(apiController.GetVisitorConfig)).Methods(http.MethodGet)

		if svr.storeSource != nil {
			subRouter.HandleFunc("/api/store/proxies", httppkg.MakeHTTPHandlerFunc(apiController.ListStoreProxies)).Methods(http.MethodGet)
			subRouter.HandleFunc("/api/store/proxies", httppkg.MakeHTTPHandlerFunc(apiController.CreateStoreProxy)).Methods(http.MethodPost)
			subRouter.HandleFunc("/api/store/proxies/{name}", httppkg.MakeHTTPHandlerFunc(apiController.GetStoreProxy)).Methods(http.MethodGet)
			subRouter.HandleFunc("/api/store/proxies/{name}", httppkg.MakeHTTPHandlerFunc(apiController.UpdateStoreProxy)).Methods(http.MethodPut)
			subRouter.HandleFunc("/api/store/proxies/{name}", httppkg.MakeHTTPHandlerFunc(apiController.DeleteStoreProxy)).Methods(http.MethodDelete)
			subRouter.HandleFunc("/api/store/visitors", httppkg.MakeHTTPHandlerFunc(apiController.ListStoreVisitors)).Methods(http.MethodGet)
			subRouter.HandleFunc("/api/store/visitors", httppkg.MakeHTTPHandlerFunc(apiController.CreateStoreVisitor)).Methods(http.MethodPost)
			subRouter.HandleFunc("/api/store/visitors/{name}", httppkg.MakeHTTPHandlerFunc(apiController.GetStoreVisitor)).Methods(http.MethodGet)
			subRouter.HandleFunc("/api/store/visitors/{name}", httppkg.MakeHTTPHandlerFunc(apiController.UpdateStoreVisitor)).Methods(http.MethodPut)
			subRouter.HandleFunc("/api/store/visitors/{name}", httppkg.MakeHTTPHandlerFunc(apiController.DeleteStoreVisitor)).Methods(http.MethodDelete)
		}
	}

	helper.Router.Handle("/favicon.ico", http.FileServer(helper.AssetsFS)).Methods("GET")
	helper.Router.PathPrefix("/static/").Handler(
		netpkg.MakeHTTPGzipHandler(http.StripPrefix("/static/", http.FileServer(helper.AssetsFS))),
	).Methods("GET")
	helper.Router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/", http.StatusMovedPermanently)
	})
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func newAPIController(svr *Service) *adminapi.Controller {
	manager := newServiceConfigManager(svr)
	var resolveCertificate func(string, []string) (string, string, error)
	if svr.adminRuntime != nil && svr.adminRuntime.Certificates() != nil {
		resolveCertificate = func(id string, domains []string) (string, string, error) {
			material, err := svr.adminRuntime.Certificates().ResolveAvailable(id, domains)
			if err != nil {
				return "", "", err
			}
			return material.CertPath, material.KeyPath, nil
		}
	}
	return adminapi.NewController(adminapi.ControllerParams{
		ServerAddr:         svr.common.ServerAddr,
		Manager:            manager,
		ResolveCertificate: resolveCertificate,
	})
}

// getAllProxyStatus returns all proxy statuses.
func (svr *Service) getAllProxyStatus() []*proxy.WorkingStatus {
	svr.ctlMu.RLock()
	ctl := svr.ctl
	svr.ctlMu.RUnlock()
	if ctl == nil {
		return nil
	}
	return ctl.pm.GetAllProxyStatus()
}

// getAllVisitorStatus returns all visitor statuses.
func (svr *Service) getAllVisitorStatus() []*visitor.WorkingStatus {
	svr.ctlMu.RLock()
	ctl := svr.ctl
	svr.ctlMu.RUnlock()
	if ctl == nil {
		return nil
	}
	return ctl.vm.GetAllVisitorStatus()
}
