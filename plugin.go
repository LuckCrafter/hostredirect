package hostredirect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/oauth2"
	"github.com/robinbraemer/event"

	"go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/edition/java/lite"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

// Config holds the server mappings
type Config struct {
	ServerMappings map[string]string
}

type Client struct {
	ctx            context.Context
	baseUrl        string
	clientId       string
	clientSecret   string
	token          *oauth2.TokenResponse
	tokenExpiresAt time.Time
}

// Plugin is a demo plugin that redirects players based on the host they connect with.
var Plugin = proxy.Plugin{
	Name: "HostRedirect",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		// Get the logger for this plugin.
		log := logr.FromContextOrDiscard(ctx)

		log.Info("HostRedirect Initlize...")
		baseurl := os.Getenv("HOSTREDIRECT_URL")
		clientId := os.Getenv("HOSTREDIRECT_CLIENTID")
		clientSecret := os.Getenv("HOSTREDIRECT_CLIENTSECRET")
		client := NewClient(ctx, baseurl, clientId, clientSecret)
		event.Subscribe(p.Event(), 0, onPlayerChooseInitialServer(p, log, client))

		log.Info("HostRedirect Initlized")
		return nil
	},
}

// onPlayerChooseInitialServer handles the PlayerChooseInitialServerEvent to redirect players.
func onPlayerChooseInitialServer(p *proxy.Proxy, log logr.Logger, c *Client) func(*proxy.PlayerChooseInitialServerEvent) {
	return func(e *proxy.PlayerChooseInitialServerEvent) {
		// Get the player's connecting host.
		conn := e.Player().VirtualHost()
		host := conn.String()
		host = lite.ClearVirtualHost(host)

		config := c.getConfig()

		// Check if we have a server mapping for this host.
		serverName, ok := config.ServerMappings[host]
		if !ok {
			msg := fmt.Sprintf("No server mapping for host: %s", host)
			log.Info(msg)
			return
		}

		// Get the registered server by name.
		var server proxy.RegisteredServer
		for _, s := range p.Servers() {
			if s.ServerInfo().Addr().String() == serverName {
				server = s
				break
			}
		}

		if server == nil {
			msg := fmt.Sprintf("Server not found: %s", serverName)
			log.Info(msg)
			e.Player().Disconnect(&component.Text{Content: msg})
			return
		}

		// Redirect the player to the appropriate server.
		e.SetInitialServer(server)
		log.Info("Redirecting player", "username", e.Player().Username(), "server", serverName)
	}
}

func NewClient(ctx context.Context, baseUrl, clientId, clientSecret string) *Client {
	return &Client{
		ctx:          ctx,
		baseUrl:      baseUrl,
		clientId:     clientId,
		clientSecret: clientSecret,
	}
}

func (c *Client) RequestToken() (*oauth2.TokenResponse, error) {
	form := url.Values{
		"client_id":     {c.clientId},
		"client_secret": {c.clientSecret},
		"grant_type":    {"client_credentials"},
	}

	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, fmt.Sprintf("%s/oauth2/token", c.baseUrl), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var token oauth2.TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}

	return &token, nil
}

func (c *Client) RenewToken() (*oauth2.TokenResponse, error) {
	if c.token != nil && time.Now().Before(c.tokenExpiresAt) {
		return c.token, nil
	}

	tok, err := c.RequestToken()
	if err != nil {
		return nil, err
	}

	c.token = tok
	c.tokenExpiresAt = time.Now().Add(time.Second * time.Duration(tok.ExpiresIn))

	return c.token, nil
}

func (c *Client) ServerSearch() (*models.ServerSearchResponse, error) {
	token, err := c.RenewToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, fmt.Sprintf("%s/api/servers", c.baseUrl), nil)
	if err != nil {
		return nil, fmt.Errorf("creating server search request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", token.TokenType, token.AccessToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing server search request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var search *models.ServerSearchResponse
	if err := json.Unmarshal(body, &search); err != nil {
		return nil, err
	}

	return search, nil
}

type Value struct {
	Value string `json:"value"`
}

type Domain struct {
	Domain Value `json:"gate.hostredirect.domain"`
}

type Data struct {
	Data Domain `json:"data"`
}

func (c *Client) ServerData(serverID string) (*Data, error) {
	token, err := c.RenewToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, fmt.Sprintf("%s/api/servers/%s/data", c.baseUrl, serverID), nil)
	if err != nil {
		return nil, fmt.Errorf("creating server search request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", token.TokenType, token.AccessToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing server search request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data Data
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	if data.Data.Domain.Value == "" {
		return nil, fmt.Errorf("Empty")
	}

	return &data, nil
}

func (c *Client) getConfig() Config {
	var config Config
	config.ServerMappings = map[string]string{}
	search, err := c.ServerSearch()
	if err != nil {
		return config // maybe fallback file
	}
	for _, server := range search.Servers {
		data, err := c.ServerData(server.Identifier)
		if err != nil {
			continue
		}
		config.ServerMappings[data.Data.Domain.Value] = server.Identifier
	}
	return config
}
