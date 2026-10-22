package client

import (
	"path/filepath"
	"sort"

	v1 "github.com/fatedier/frp/pkg/config/v1"
)

func (svr *Service) certificateReferences(id, certPath, keyPath string) []string {
	svr.reloadMu.Lock()
	configSource := svr.configSource
	storeSource := svr.storeSource
	svr.reloadMu.Unlock()

	proxies := make([]v1.ProxyConfigurer, 0)
	if configSource != nil {
		proxies = append(proxies, configSource.GetAllProxies()...)
	}
	if storeSource != nil {
		storeProxies, err := storeSource.GetAllProxies()
		if err == nil {
			proxies = append(proxies, storeProxies...)
		}
	}

	names := make(map[string]struct{})
	certPath = filepath.Clean(certPath)
	keyPath = filepath.Clean(keyPath)
	for _, proxyConfig := range proxies {
		base := proxyConfig.GetBaseConfig()
		if id != "" && base.Metadatas[v1.CertificateIDMetadataKey] == id {
			names[base.Name] = struct{}{}
			continue
		}
		if certPath == "" || keyPath == "" {
			continue
		}
		if proxyCertPath, proxyKeyPath, ok := certificatePluginPaths(base); ok &&
			filepath.Clean(proxyCertPath) == certPath && filepath.Clean(proxyKeyPath) == keyPath {
			names[base.Name] = struct{}{}
		}
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func certificatePluginPaths(base *v1.ProxyBaseConfig) (string, string, bool) {
	switch options := base.Plugin.ClientPluginOptions.(type) {
	case *v1.HTTPS2HTTPPluginOptions:
		return options.CrtPath, options.KeyPath, true
	case *v1.HTTPS2HTTPSPluginOptions:
		return options.CrtPath, options.KeyPath, true
	case *v1.TLS2RawPluginOptions:
		return options.CrtPath, options.KeyPath, true
	default:
		return "", "", false
	}
}
