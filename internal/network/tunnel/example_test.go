// Example usage of the tunnel module
// This file demonstrates how to use the tunnel module for remote access

package tunnel_test

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"nas-os/internal/network/tunnel"
)

// Example_basicUsage demonstrates basic tunnel usage
func Example_basicUsage() {
	// Create configuration with default values
	config := tunnel.DefaultConfig()
	config.ListenPort = 51820
	
	// Add STUN servers for NAT detection
	config.STUNServers = []string{
		"stun:stun.l.google.com:19302",
		"stun:stun.cloudflare.com:3478",
	}
	
	// Create tunnel manager
	manager, err := tunnel.NewTunnelManager(config)
	if err != nil {
		log.Fatalf("Failed to create tunnel manager: %v", err)
	}
	
	// Register event handler
	manager.OnEvent(func(event tunnel.TunnelEvent) {
		switch event.Type {
		case "started":
			fmt.Println("Tunnel started successfully")
		case "nat_discovered":
			if result, ok := event.Data.(*tunnel.STUNResult); ok {
				fmt.Printf("NAT Type: %s\n", result.NATType)
				fmt.Printf("Public Address: %s:%d\n", result.PublicIP, result.PublicPort)
			}
		case "peer_added":
			fmt.Printf("New peer connected: %v\n", event.Data)
		}
	})
	
	// Start the tunnel
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		log.Fatalf("Failed to start tunnel: %v", err)
	}
	defer manager.Close()
	
	fmt.Printf("Local Peer ID: %s\n", manager.GetPeerID())
	fmt.Printf("Public Key: %x\n", manager.GetPublicKey())
}

// Example_connectToPeer demonstrates connecting to a remote peer
func Example_connectToPeer() {
	config := tunnel.DefaultConfig()
	config.SignalingURL = "wss://signal.example.com/ws"
	
	manager, _ := tunnel.NewTunnelManager(config)
	ctx := context.Background()
	manager.Start(ctx)
	defer manager.Close()
	
	// Create peer info
	peerInfo := &tunnel.PeerInfo{
		ID:        "remote-nas-device",
		PublicKey: []byte{0x01, 0x02, 0x03}, // Peer's actual public key
		NATType:   tunnel.NATPortRestricted,
		Endpoints: []net.UDPAddr{
			{IP: net.ParseIP("192.168.1.100"), Port: 51820},
		},
	}
	
	// Connect to peer
	if err := manager.ConnectPeer(ctx, "remote-nas-device", peerInfo); err != nil {
		log.Printf("Failed to connect: %v", err)
		return
	}
	
	// Send data
	manager.Send("remote-nas-device", []byte("Hello from NAS!"))
	
	// Receive data
	for data := range manager.Receive() {
		fmt.Printf("Received from %s: %s\n", data.PeerID, string(data.Data))
	}
}

// Example_stunDiscovery demonstrates NAT type detection
func Example_stunDiscovery() {
	config := tunnel.DefaultConfig()
	stunClient := tunnel.NewSTUNClient(config)
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	result, err := stunClient.Discover(ctx)
	if err != nil {
		log.Fatalf("STUN discovery failed: %v", err)
	}
	
	fmt.Printf("Local IP: %s\n", result.LocalIP)
	fmt.Printf("Public IP: %s\n", result.PublicIP)
	fmt.Printf("Public Port: %d\n", result.PublicPort)
	fmt.Printf("NAT Type: %s\n", result.NATType)
	
	// Determine if TURN relay is needed
	if result.NATType == tunnel.NATSymmetric {
		fmt.Println("Symmetric NAT detected - TURN relay recommended")
	}
}

// Example_turnRelay demonstrates using TURN relay
func Example_turnRelay() {
	config := tunnel.DefaultConfig()
	
	turnClient := tunnel.NewTURNClient(config, tunnel.TURNServer{
		URL:      "turn:turn.example.com:3478",
		Username: "user",
		Password: "password",
	})
	
	ctx := context.Background()
	
	// Connect to TURN server
	if err := turnClient.Connect(ctx, "turn.example.com:3478"); err != nil {
		log.Fatalf("TURN connection failed: %v", err)
	}
	
	// Allocate relay
	allocCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	allocation, err := turnClient.Allocate(allocCtx)
	if err != nil {
		log.Fatalf("TURN allocation failed: %v", err)
	}
	
	fmt.Printf("Relay address: %s\n", allocation.RelayAddr)
	
	// Create permission for peer
	peerAddr := &net.UDPAddr{IP: net.ParseIP("1.2.3.4"), Port: 51820}
	if err := turnClient.CreatePermission(ctx, peerAddr); err != nil {
		log.Printf("Permission failed: %v", err)
	}
	
	// Send data through relay
	turnClient.Send(ctx, []byte("Hello via relay"), peerAddr)
	
	// Receive data
	data, from, _ := turnClient.Receive(ctx)
	fmt.Printf("Received %d bytes from %s\n", len(data), from)
}

// Example_encryption demonstrates end-to-end encryption
func Example_encryption() {
	// Create crypto instance
	crypto, err := tunnel.NewCrypto(&tunnel.CryptoConfig{
		CipherType: tunnel.CipherChaCha20Poly1305,
	})
	if err != nil {
		log.Fatal(err)
	}
	
	// Generate key pair
	crypto.GenerateKeyPair()
	localPublicKey := crypto.GetPublicKey()
	
	// In real scenario, exchange public keys via signaling
	// peerPublicKey would be received from remote peer
	peerPublicKey := make([]byte, 32)
	
	// Derive shared key
	sharedKey, err := crypto.DeriveSharedKey(peerPublicKey)
	if err != nil {
		log.Fatal(err)
	}
	
	// Set session key for peer
	crypto.SetPeerKey("peer-123", sharedKey)
	
	// Encrypt data
	plaintext := []byte("Secret message")
	ciphertext, err := crypto.Encrypt(plaintext, "peer-123")
	if err != nil {
		log.Fatal(err)
	}
	
	// Decrypt data
	decrypted, err := crypto.Decrypt(ciphertext, "peer-123")
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Original: %s\n", plaintext)
	fmt.Printf("Decrypted: %s\n", decrypted)
	fmt.Printf("Public Key: %x\n", localPublicKey)
}

// Example_customConfig demonstrates custom configuration
func Example_customConfig() {
	// Build custom configuration
	config, err := tunnel.NewConfigBuilder().
		WithListenPort(51821).
		WithSTUNServers(
			"stun:stun.l.google.com:19302",
			"stun:stun1.l.google.com:19302",
		).
		WithTURNServer("turn:turn.example.com:3478", "user", "pass").
		WithSignaling("wss://signal.example.com/ws").
		WithTimeouts(5*time.Second, 10*time.Second, 30*time.Second).
		WithKeepalive(30 * time.Second).
		WithMaxPeers(50).
		Build()
	
	if err != nil {
		log.Fatalf("Invalid config: %v", err)
	}
	
	// Save config to file
	if err := tunnel.SaveConfig("/etc/nas-os/tunnel.json", config); err != nil {
		log.Printf("Failed to save config: %v", err)
	}
	
	// Load config from file
	loadedConfig, err := tunnel.LoadConfig("/etc/nas-os/tunnel.json")
	if err != nil {
		log.Printf("Failed to load config: %v", err)
	}
	
	fmt.Printf("Listen Port: %d\n", loadedConfig.ListenPort)
	fmt.Printf("Max Peers: %d\n", loadedConfig.MaxPeers)
}

// Example_signalingServer demonstrates running a signaling server
func Example_signalingServer() {
	server := tunnel.NewSignalingServer(8080)
	
	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		log.Fatalf("Failed to start signaling server: %v", err)
	}
	defer server.Stop()
	
	fmt.Println("Signaling server running on port 8080")
	fmt.Printf("Connected peers: %v\n", server.GetPeers())
}