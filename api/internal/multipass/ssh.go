package multipass

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

const (
	defaultSSHUser = "ubuntu"
	sshKeyCacheDir = ".cloudpass"
	knownHostsFile = "known_hosts"
)

type SSHClientInterface interface {
	Connect(sshKeyPath, ip string, timeoutSec int) error
	Close() error
	OpenTerminal(rows, cols int) (*ssh.Session, error)
}

type RealSSHClient struct {
	client  *ssh.Client
	session *ssh.Session
}

func NewSSHClient(timeoutSec int) SSHClientInterface {
	return &RealSSHClient{}
}

func (c *RealSSHClient) Connect(sshKeyPath, ip string, timeoutSec int) error {
	keyPath, err := findSSHKey(sshKeyPath)
	if err != nil {
		return fmt.Errorf("failed to find SSH key: %w", err)
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to parse SSH key: %w", err)
	}

	hostKeyCB, err := hostKeyCallback()
	if err != nil {
		hostKeyCB = ssh.InsecureIgnoreHostKey()
	}

	config := &ssh.ClientConfig{
		User: defaultSSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCB,
	}

	addr := fmt.Sprintf("%s:22", ip)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	c.client = client
	return nil
}

func (c *RealSSHClient) Close() error {
	if c.session != nil {
		c.session.Close()
		c.session = nil
	}
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

func (c *RealSSHClient) OpenTerminal(rows, cols int) (*ssh.Session, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		session.Close()
		return nil, fmt.Errorf("failed to request PTY: %w", err)
	}

	c.session = session
	return session, nil
}

type MockSSHClient struct {
	ConnectFunc      func(sshKeyPath, ip string, timeoutSec int) error
	CloseFunc        func() error
	OpenTerminalFunc func(rows, cols int) (*ssh.Session, error)
}

func (m *MockSSHClient) Connect(sshKeyPath, ip string, timeoutSec int) error {
	if m.ConnectFunc != nil {
		return m.ConnectFunc(sshKeyPath, ip, timeoutSec)
	}
	return nil
}

func (m *MockSSHClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *MockSSHClient) OpenTerminal(rows, cols int) (*ssh.Session, error) {
	if m.OpenTerminalFunc != nil {
		return m.OpenTerminalFunc(rows, cols)
	}
	return nil, fmt.Errorf("mock terminal not implemented")
}

func findSSHKey(sshKeyPath string) (string, error) {
	if sshKeyPath != "" {
		if _, err := os.Stat(sshKeyPath); err == nil {
			log.Debug().Str("path", sshKeyPath).Msg("found SSH key from config")
			return sshKeyPath, nil
		}
	}

	home := os.Getenv("HOME")
	userKeyPath := filepath.Join(home, ".cloudpass", "multipass_id_rsa")
	if _, err := os.Stat(userKeyPath); err == nil {
		log.Debug().Str("path", userKeyPath).Msg("found SSH key at default user path")
		return userKeyPath, nil
	}

	return "", fmt.Errorf("SSH key not found. Please ensure the SSH key has been copied to ~/.cloudpass/multipass_id_rsa")
}

func getKnownHostsPath() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	if home == "" {
		return "", fmt.Errorf("cannot determine home directory")
	}

	cacheDir := filepath.Join(home, sshKeyCacheDir)
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	return filepath.Join(cacheDir, knownHostsFile), nil
}

func hostKeyCallback() (ssh.HostKeyCallback, error) {
	knownHostsPath, err := getKnownHostsPath()
	if err != nil {
		log.Warn().Err(err).Msg("failed to get known_hosts path, using insecure mode")
		return ssh.InsecureIgnoreHostKey(), nil
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		keyBytes := base64.StdEncoding.EncodeToString(key.Marshal())

		if data, err := os.ReadFile(knownHostsPath); err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				line := scanner.Text()
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					storedHostname := parts[0]
					storedKey := parts[1]
					if storedHostname == hostname {
						if storedKey == keyBytes {
							return nil
						}
						return fmt.Errorf("host key mismatch for %s", hostname)
					}
				}
			}
		}

		f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Warn().Err(err).Msg("failed to cache host key")
			return nil
		}
		defer f.Close()

		_, err = fmt.Fprintf(f, "%s %s\n", hostname, keyBytes)
		if err != nil {
			log.Warn().Err(err).Msg("failed to write host key to cache")
		}

		log.Info().Str("hostname", hostname).Msg("accepted new host key")
		return nil
	}, nil
}
