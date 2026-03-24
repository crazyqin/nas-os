package tunnel

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

// Config errors
var (
	ErrInvalidConfigFile = errors.New("invalid configuration file")
	ErrMissingSTUNServer = errors.New("at least one STUN server required")
)

// LoadConfig loads tunnel configuration from a file
func LoadConfig(path string) (*TunnelConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, ErrInvalidConfigFile
	}

	if err := ValidateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// SaveConfig saves tunnel configuration to a file
func SaveConfig(path string, config *TunnelConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// ValidateConfig validates tunnel configuration
func ValidateConfig(config *TunnelConfig) error {
	if config == nil {
		return errors.New("config is nil")
	}

	if len(config.STUNServers) == 0 {
		return ErrMissingSTUNServer
	}

	if config.ListenPort < 0 || config.ListenPort > 65535 {
		return errors.New("invalid listen port")
	}

	if config.STUNTimeout <= 0 {
		config.STUNTimeout = 5 * time.Second
	}

	if config.TURNTimeout <= 0 {
		config.TURNTimeout = 10 * time.Second
	}

	if config.ICETimeout <= 0 {
		config.ICETimeout = 30 * time.Second
	}

	if config.Keepalive <= 0 {
		config.Keepalive = 25 * time.Second
	}

	if config.MaxPeers <= 0 {
		config.MaxPeers = 100
	}

	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}

	return nil
}

// GenerateKeyPair generates a new encryption key pair
func GenerateKeyPair() (publicKey, privateKey []byte, err error) {
	privateKey = make([]byte, 32)
	if _, err := rand.Read(privateKey); err != nil {
		return nil, nil, err
	}

	publicKey = make([]byte, 32)
	if _, err := rand.Read(publicKey); err != nil {
		return nil, nil, err
	}

	return publicKey, privateKey, nil
}

// GenerateEncryptionKey generates a random encryption key
func GenerateEncryptionKey() ([]byte, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	return key, err
}

// ConfigBuilder helps build tunnel configuration
type ConfigBuilder struct {
	config *TunnelConfig
}

// NewConfigBuilder creates a new config builder
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: DefaultConfig(),
	}
}

// WithListenPort sets the listen port
func (b *ConfigBuilder) WithListenPort(port int) *ConfigBuilder {
	b.config.ListenPort = port
	return b
}

// WithSTUNServers sets STUN servers
func (b *ConfigBuilder) WithSTUNServers(servers ...string) *ConfigBuilder {
	b.config.STUNServers = servers
	return b
}

// WithTURNServer adds a TURN server
func (b *ConfigBuilder) WithTURNServer(url, username, password string) *ConfigBuilder {
	b.config.TURNServers = append(b.config.TURNServers, TURNServer{
		URL:      url,
		Username: username,
		Password: password,
	})
	return b
}

// WithSignaling sets the signaling server URL
func (b *ConfigBuilder) WithSignaling(url string) *ConfigBuilder {
	b.config.SignalingURL = url
	return b
}

// WithTimeouts sets various timeouts
func (b *ConfigBuilder) WithTimeouts(stun, turn, ice time.Duration) *ConfigBuilder {
	b.config.STUNTimeout = stun
	b.config.TURNTimeout = turn
	b.config.ICETimeout = ice
	return b
}

// WithKeepalive sets the keepalive interval
func (b *ConfigBuilder) WithKeepalive(interval time.Duration) *ConfigBuilder {
	b.config.Keepalive = interval
	return b
}

// WithMaxPeers sets the maximum number of peers
func (b *ConfigBuilder) WithMaxPeers(max int) *ConfigBuilder {
	b.config.MaxPeers = max
	return b
}

// WithEncryptionKey sets the encryption key
func (b *ConfigBuilder) WithEncryptionKey(key []byte) *ConfigBuilder {
	b.config.EncryptionKey = key
	return b
}

// Build returns the built configuration
func (b *ConfigBuilder) Build() (*TunnelConfig, error) {
	if err := ValidateConfig(b.config); err != nil {
		return nil, err
	}
	return b.config, nil
}

// PublicSTUNServers returns a list of public STUN servers
func PublicSTUNServers() []string {
	return []string{
		"stun:stun.l.google.com:19302",
		"stun:stun1.l.google.com:19302",
		"stun:stun2.l.google.com:19302",
		"stun:stun3.l.google.com:19302",
		"stun:stun4.l.google.com:19302",
		"stun:stun.cloudflare.com:3478",
		"stun:stun.e estimation.com:3478",
		"stun:stun.schlund.de:3478",
	}
}

// PeerConfigJSON represents peer configuration in JSON format
type PeerConfigJSON struct {
	ID        string   `json:"id"`
	PublicKey string   `json:"public_key"`
	Endpoints []string `json:"endpoints"`
}

// ParsePeerConfig parses peer configuration from JSON
func ParsePeerConfig(data []byte) (*PeerConfig, error) {
	var jsonConfig PeerConfigJSON
	if err := json.Unmarshal(data, &jsonConfig); err != nil {
		return nil, err
	}

	publicKey, err := hex.DecodeString(jsonConfig.PublicKey)
	if err != nil {
		return nil, err
	}

	config := &PeerConfig{
		ID:        jsonConfig.ID,
		PublicKey: publicKey,
		Endpoints: make([]*net.UDPAddr, 0),
	}

	for _, ep := range jsonConfig.Endpoints {
		addr, err := net.ResolveUDPAddr("udp", ep)
		if err != nil {
			continue
		}
		config.Endpoints = append(config.Endpoints, addr)
	}

	return config, nil
}

// ToJSON converts peer configuration to JSON
func (c *PeerConfig) ToJSON() ([]byte, error) {
	jsonConfig := PeerConfigJSON{
		ID:        c.ID,
		PublicKey: hex.EncodeToString(c.PublicKey),
		Endpoints: make([]string, 0),
	}

	for _, ep := range c.Endpoints {
		jsonConfig.Endpoints = append(jsonConfig.Endpoints, ep.String())
	}

	return json.MarshalIndent(jsonConfig, "", "  ")
}

// TunnelConfigJSON represents tunnel configuration in JSON format
type TunnelConfigJSON struct {
	ListenPort   int          `json:"listen_port"`
	STUNServers  []string     `json:"stun_servers"`
	TURNServers  []TURNServer `json:"turn_servers"`
	SignalingURL string       `json:"signaling_url"`
	STUNTimeout  string       `json:"stun_timeout"`
	TURNTimeout  string       `json:"turn_timeout"`
	ICETimeout   string       `json:"ice_timeout"`
	Keepalive    string       `json:"keepalive"`
	MaxPeers     int          `json:"max_peers"`
	MaxRetries   int          `json:"max_retries"`
}

// ToJSON converts tunnel configuration to JSON
func (c *TunnelConfig) ToJSON() ([]byte, error) {
	jsonConfig := TunnelConfigJSON{
		ListenPort:   c.ListenPort,
		STUNServers:  c.STUNServers,
		TURNServers:  c.TURNServers,
		SignalingURL: c.SignalingURL,
		STUNTimeout:  c.STUNTimeout.String(),
		TURNTimeout:  c.TURNTimeout.String(),
		ICETimeout:   c.ICETimeout.String(),
		Keepalive:    c.Keepalive.String(),
		MaxPeers:     c.MaxPeers,
		MaxRetries:   c.MaxRetries,
	}

	return json.MarshalIndent(jsonConfig, "", "  ")
}

// Environment variable names
const (
	EnvTunnelListenPort   = "TUNNEL_LISTEN_PORT"
	EnvTunnelSTUNServers  = "TUNNEL_STUN_SERVERS"
	EnvTunnelTURNServer   = "TUNNEL_TURN_SERVER"
	EnvTunnelSignalingURL = "TUNNEL_SIGNALING_URL"
	EnvTunnelMaxPeers     = "TUNNEL_MAX_PEERS"
)

// LoadConfigFromEnv loads configuration from environment variables
func LoadConfigFromEnv() *TunnelConfig {
	config := DefaultConfig()

	if port := os.Getenv(EnvTunnelListenPort); port != "" {
		var p int
		if _, err := fmt.Sscanf(port, "%d", &p); err == nil {
			config.ListenPort = p
		}
	}

	if servers := os.Getenv(EnvTunnelSTUNServers); servers != "" {
		// Parse comma-separated list
		var serverList []string
		if err := json.Unmarshal([]byte(servers), &serverList); err == nil {
			config.STUNServers = serverList
		}
	}

	if url := os.Getenv(EnvTunnelSignalingURL); url != "" {
		config.SignalingURL = url
	}

	if maxPeers := os.Getenv(EnvTunnelMaxPeers); maxPeers != "" {
		var m int
		if _, err := fmt.Sscanf(maxPeers, "%d", &m); err == nil {
			config.MaxPeers = m
		}
	}

	return config
}
