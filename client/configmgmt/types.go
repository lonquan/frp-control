package configmgmt

import (
	"errors"
	"time"

	"github.com/fatedier/frp/client/proxy"
	"github.com/fatedier/frp/client/visitor"
	v1 "github.com/fatedier/frp/pkg/config/v1"
)

var (
	ErrInvalidArgument = errors.New("invalid argument")
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrStoreDisabled   = errors.New("store disabled")
	ErrApplyConfig     = errors.New("apply config failed")
)

const (
	ConfigSourceFile  = "config"
	ConfigSourceStore = "store"
)

type ProxyConfigEntry struct {
	Config v1.ProxyConfigurer
	Source string
}

type VisitorConfigEntry struct {
	Config v1.VisitorConfigurer
	Source string
}

type ConfigManager interface {
	ReloadFromFile(strict bool) error

	ReadConfigFile() (string, error)
	WriteConfigFile(content []byte) error

	GetProxyStatus() []*proxy.WorkingStatus
	GetVisitorStatus() []*visitor.WorkingStatus
	IsStoreProxyEnabled(name string) bool
	StoreEnabled() bool

	GetProxyConfig(name string) (v1.ProxyConfigurer, bool)
	GetVisitorConfig(name string) (v1.VisitorConfigurer, bool)
	ListProxyConfigs() ([]ProxyConfigEntry, error)
	ListVisitorConfigs() ([]VisitorConfigEntry, error)

	ListStoreProxies() ([]v1.ProxyConfigurer, error)
	GetStoreProxy(name string) (v1.ProxyConfigurer, error)
	CreateStoreProxy(cfg v1.ProxyConfigurer) (v1.ProxyConfigurer, error)
	UpdateStoreProxy(name string, cfg v1.ProxyConfigurer) (v1.ProxyConfigurer, error)
	DeleteStoreProxy(name string) error

	ListStoreVisitors() ([]v1.VisitorConfigurer, error)
	GetStoreVisitor(name string) (v1.VisitorConfigurer, error)
	CreateStoreVisitor(cfg v1.VisitorConfigurer) (v1.VisitorConfigurer, error)
	UpdateStoreVisitor(name string, cfg v1.VisitorConfigurer) (v1.VisitorConfigurer, error)
	DeleteStoreVisitor(name string) error

	GracefulClose(d time.Duration)
}
