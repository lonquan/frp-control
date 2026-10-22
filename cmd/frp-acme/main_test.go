package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateJobRequiresDNSProvider(t *testing.T) {
	err := validateJob(job{
		Domains:     []string{"example.com"},
		CertPath:    "/tmp/cert.pem",
		KeyPath:     "/tmp/key.pem",
		AccountPath: "/tmp/account",
	})
	require.ErrorContains(t, err, "DNS provider is required")
}

func TestValidateJobAcceptsWildcardDNSJob(t *testing.T) {
	require.NoError(t, validateJob(job{
		Domains:     []string{"example.com", "*.example.com"},
		Provider:    "cloudflare",
		CertPath:    "/tmp/cert.pem",
		KeyPath:     "/tmp/key.pem",
		AccountPath: "/tmp/account",
	}))
}
