package client

import (
	"errors"
	"sync"
	"sync/atomic"

	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

var (
	ErrProxyBlocked = errors.New("proxy blocked")
	proxyList       []string
	proxyListMutex  sync.Mutex
	proxyCounter    uint32
)

type ProxiedClient struct {
	tls_client.HttpClient
	ProxyURL string
}

func SetProxyList(proxies []string) {
	proxyListMutex.Lock()
	defer proxyListMutex.Unlock()
	proxyList = proxies
}

func RemoveProxy(proxyURL string) int {
	proxyListMutex.Lock()
	defer proxyListMutex.Unlock()
	
	if proxyURL != "" {
		for i, p := range proxyList {
			if p == proxyURL {
				proxyList = append(proxyList[:i], proxyList[i+1:]...)
				break
			}
		}
	}
	return len(proxyList)
}

func CreateClient() (*ProxiedClient, error) {
	jar := tls_client.NewCookieJar()
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_120),
		tls_client.WithNotFollowRedirects(),
		tls_client.WithCookieJar(jar),
	}
	
	var proxyURL string
	proxyListMutex.Lock()
	if len(proxyList) > 0 {
		idx := atomic.AddUint32(&proxyCounter, 1)
		proxyURL = proxyList[int(idx-1)%len(proxyList)]
		options = append(options, tls_client.WithProxyUrl(proxyURL))
	}
	proxyListMutex.Unlock()
	
	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		return nil, err
	}
	
	return &ProxiedClient{HttpClient: client, ProxyURL: proxyURL}, nil
}