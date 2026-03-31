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

type SSHClient struct {
	client  *ssh.Client
	session *ssh.Session
}

func NewSSHClient(timeoutSec int) *SSHClient {
	return &SSHClient{}
}

func (c *multipassClient) GetInstanceIP(name string) (string, error) {
	instance, err := c.GetInstance(name)
	if err != nil {
		return "", err
	}

	if len(instance.IPv4) == 0 {
		return "", fmt.Errorf("instance has no IPv4 address")
	}

	return instance.IPv4[0], nil
}

func findSSHKey(sshKeyPath string) (string, error) {
	// 1. Check config-specified path first
	if sshKeyPath != "" {
		if _, err := os.Stat(sshKeyPath); err == nil {
			log.Debug().Str("path", sshKeyPath).Msg("found SSH key from config")
			return sshKeyPath, nil
		}
	}

	// 2. Check default user path
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
							return nil // Key matches, accept
						}
						return fmt.Errorf("host key mismatch for %s", hostname)
					}
				}
			}
		}

		// New host key - accept and cache it
		f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Warn().Err(err).Msg("failed to cache host key")
			return nil // Accept anyway
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

func (s *SSHClient) Connect(sshKeyPath, ip string, timeoutSec int) error {
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

	s.client = client
	return nil
}

func (s *SSHClient) OpenTerminal(rows, cols int) (*ssh.Session, error) {
	if s.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	session, err := s.client.NewSession()
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

	s.session = session
	return session, nil
}

func (s *SSHClient) Close() {
	if s.session != nil {
		s.session.Close()
		s.session = nil
	}
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}
}
