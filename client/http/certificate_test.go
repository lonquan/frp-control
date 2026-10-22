package http

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fatedier/frp/client/http/model"
	v1 "github.com/fatedier/frp/pkg/config/v1"
)

func TestPrepareCertificateInjectsManagedPaths(t *testing.T) {
	controller := &Controller{
		resolveCertificate: func(id string, domains []string) (string, string, error) {
			require.Equal(t, "cert-123", id)
			require.Equal(t, []string{"api.example.com"}, domains)
			return "/var/lib/frp/acme/cert.pem", "/var/lib/frp/acme/key.pem", nil
		},
	}
	payload := &model.ProxyDefinition{
		Name:          "api",
		Type:          "https",
		CertificateID: "cert-123",
		HTTPS: &v1.HTTPSProxyConfig{
			DomainConfig: v1.DomainConfig{CustomDomains: []string{"api.example.com"}},
			ProxyBaseConfig: v1.ProxyBaseConfig{
				ProxyBackend: v1.ProxyBackend{
					Plugin: v1.TypedClientPluginOptions{
						Type: v1.PluginHTTPS2HTTP,
						ClientPluginOptions: &v1.HTTPS2HTTPPluginOptions{
							Type:      v1.PluginHTTPS2HTTP,
							LocalAddr: "127.0.0.1:8000",
						},
					},
				},
			},
		},
	}

	require.NoError(t, controller.prepareCertificate(payload))
	options := payload.HTTPS.Plugin.ClientPluginOptions.(*v1.HTTPS2HTTPPluginOptions)
	require.Equal(t, "/var/lib/frp/acme/cert.pem", options.CrtPath)
	require.Equal(t, "/var/lib/frp/acme/key.pem", options.KeyPath)
	require.Equal(t, "cert-123", payload.HTTPS.Metadatas[v1.CertificateIDMetadataKey])
}

func TestPrepareManualCertificateClearsManagedReference(t *testing.T) {
	controller := &Controller{}
	payload := &model.ProxyDefinition{
		Name: "api",
		Type: "https",
		HTTPS: &v1.HTTPSProxyConfig{
			DomainConfig: v1.DomainConfig{CustomDomains: []string{"api.example.com"}},
			ProxyBaseConfig: v1.ProxyBaseConfig{
				Metadatas: map[string]string{v1.CertificateIDMetadataKey: "old"},
				ProxyBackend: v1.ProxyBackend{
					Plugin: v1.TypedClientPluginOptions{
						Type: v1.PluginHTTPS2HTTP,
						ClientPluginOptions: &v1.HTTPS2HTTPPluginOptions{
							CrtPath: "/manual/cert.pem",
							KeyPath: "/manual/key.pem",
						},
					},
				},
			},
		},
	}

	require.NoError(t, controller.prepareCertificate(payload))
	require.NotContains(t, payload.HTTPS.Metadatas, v1.CertificateIDMetadataKey)
	options := payload.HTTPS.Plugin.ClientPluginOptions.(*v1.HTTPS2HTTPPluginOptions)
	require.Equal(t, "/manual/cert.pem", options.CrtPath)
}
