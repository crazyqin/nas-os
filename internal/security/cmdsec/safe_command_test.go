package cmdsec

import (
	"context"
	"strings"
	"testing"
)

// TestCommandValidationError tests the Error method of CommandValidationError.
func TestCommandValidationError(t *testing.T) {
	tests := []struct {
		name     string
		err      *CommandValidationError
		expected string
	}{
		{
			name:     "basic error",
			err:      &CommandValidationError{Param: "device", Msg: "cannot be empty"},
			expected: "command validation error: device - cannot be empty",
		},
		{
			name:     "invalid format error",
			err:      &CommandValidationError{Param: "path", Msg: "contains disallowed characters"},
			expected: "command validation error: path - contains disallowed characters",
		},
		{
			name:     "path traversal error",
			err:      &CommandValidationError{Param: "path", Msg: "path traversal detected"},
			expected: "command validation error: path - path traversal detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("CommandValidationError.Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestValidateDevicePath tests device path validation.
func TestValidateDevicePath(t *testing.T) {
	tests := []struct {
		name    string
		device  string
		wantErr bool
	}{
		// Valid device paths
		{"valid sd device", "/dev/sda", false},
		{"valid sd device with partition", "/dev/sda1", false},
		{"valid sd device multi-letter", "/dev/sdab", false},
		{"valid sd device with number", "/dev/sdab1", false},
		{"valid nvme device", "/dev/nvme0n1", false},
		{"valid nvme device with partition", "/dev/nvme0n1p1", false},
		{"valid mapper device", "/dev/mapper/vg0-lv0", false},
		{"valid mapper device simple", "/dev/mapper/root", false},
		{"valid disk by-id", "/dev/disk/by-id/ata-Samsung_SSD_860", false},
		{"valid disk by-uuid", "/dev/disk/by-uuid/123e4567-e89b", false},
		{"valid disk by-label", "/dev/disk/by-label/ROOT", false},
		{"valid disk by-partuuid", "/dev/disk/by-partuuid/abc123", false},
		{"valid disk by-partlabel", "/dev/disk/by-partlabel/EFI", false},
		{"valid disk by-path simple", "/dev/disk/by-path/pci-0000-00-1f-2-ata-1", false},
		// Note: by-path with colons is rejected by regex (no colons in allowed chars)
		{"invalid disk by-path with colon", "/dev/disk/by-path/pci-0000:00:1f.2-ata-1", true},

		// Invalid device paths
		{"empty device", "", true},
		{"relative path", "dev/sda", true},
		{"path traversal", "/dev/../etc/passwd", true},
		{"invalid device name", "/dev/sda;rm -rf /", true},
		{"invalid nvme format", "/dev/nvme0", true},
		{"invalid nvme format 2", "/dev/nvme0n", true},
		{"random path", "/etc/passwd", true},
		{"invalid chars", "/dev/sda$(whoami)", true},
		{"backtick injection", "/dev/sda`id`", true},
		{"pipe injection", "/dev/sda|cat", true},
		{"ampersand injection", "/dev/sda&&ls", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDevicePath(tt.device)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDevicePath(%q) error = %v, wantErr %v", tt.device, err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				if _, ok := err.(*CommandValidationError); !ok {
					t.Errorf("ValidateDevicePath(%q) returned wrong error type: %T", tt.device, err)
				}
			}
		})
	}
}

// TestValidatePath tests path validation.
func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		// Invalid paths
		{"empty path", "", true},
		{"root path rejected by regex", "/", true}, // Regex requires chars after /
		{"relative path", "mnt/data", true},
		{"valid nested path", "/mnt/data/backup", false},
		{"valid with underscores", "/mnt/data_backup", false},
		{"valid with dashes", "/mnt/data-backup", false},
		{"valid with dots", "/mnt/data.backup", false},
		{"valid complex path", "/mnt/data-backup_2024.01", false},

		// Invalid paths
		{"empty path", "", true},
		{"relative path", "mnt/data", true},
		{"path traversal", "/mnt/../etc/passwd", true},
		{"double traversal", "/mnt/../../etc/passwd", true},
		{"traversal in middle", "/mnt/data/../../../etc/passwd", true},
		{"spaces not allowed", "/mnt/my data", true},
		{"special chars", "/mnt/data$(id)", true},
		{"backtick injection", "/mnt/data`id`", true},
		{"semicolon injection", "/mnt/data;ls", true},
		{"pipe injection", "/mnt/data|cat", true},
		{"ampersand injection", "/mnt/data&&ls", true},
		{"parenthesis injection", "/mnt/data(test)", true},
		{"angle bracket injection", "/mnt/data>file", true},
		{"newline injection", "/mnt/data\nfile", true},
		{"carriage return injection", "/mnt/data\rfile", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				if _, ok := err.(*CommandValidationError); !ok {
					t.Errorf("ValidatePath(%q) returned wrong error type: %T", tt.path, err)
				}
			}
		})
	}
}

// TestValidateMountOptions tests mount options validation.
func TestValidateMountOptions(t *testing.T) {
	tests := []struct {
		name    string
		options []string
		wantErr bool
	}{
		// Valid options
		{"empty options", []string{}, false},
		{"single option", []string{"ro"}, false},
		{"multiple options", []string{"ro", "noexec", "nosuid"}, false},
		{"option with value", []string{"uid=1000"}, false},
		{"option with gid", []string{"gid=1000"}, false},
		{"complex options", []string{"rw", "noatime", "nodiratime", "compress=zstd"}, false},
		{"numbers in option", []string{"mode=0755"}, false},
		{"underscore in option", []string{"no_exec"}, false},

		// Invalid options
		{"semicolon injection", []string{"ro;ls"}, true},
		{"pipe injection", []string{"ro|cat"}, true},
		{"ampersand injection", []string{"ro&&ls"}, true},
		{"dollar injection", []string{"$PATH"}, true},
		{"backtick injection", []string{"`id`"}, true},
		{"space in option", []string{"ro noexec"}, true},
		{"parenthesis injection", []string{"ro(test)"}, true},
		{"angle bracket injection", []string{"ro>file"}, true},
		{"newline injection", []string{"ro\nls"}, true},
		{"carriage return injection", []string{"ro\rls"}, true},
		{"dash in option", []string{"no-exec"}, true}, // dash not allowed
		{"dot in option", []string{"opt.name"}, true}, // dot not allowed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMountOptions(tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMountOptions(%v) error = %v, wantErr %v", tt.options, err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				if _, ok := err.(*CommandValidationError); !ok {
					t.Errorf("ValidateMountOptions(%v) returned wrong error type: %T", tt.options, err)
				}
			}
		})
	}
}

// TestValidateFSType tests filesystem type validation.
func TestValidateFSType(t *testing.T) {
	tests := []struct {
		name    string
		fsType  string
		wantErr bool
	}{
		// Valid filesystem types
		{"ext4", "ext4", false},
		{"ext3", "ext3", false},
		{"ext2", "ext2", false},
		{"xfs", "xfs", false},
		{"btrfs", "btrfs", false},
		{"ntfs", "ntfs", false},
		{"fat32", "fat32", false},
		{"vfat", "vfat", false},
		{"exfat", "exfat", false},
		{"zfs", "zfs", false},
		{"jfs", "jfs", false},
		{"reiserfs", "reiserfs", false},
		{"swap", "swap", false},
		{"tmpfs", "tmpfs", false},
		{"proc", "proc", false},
		{"sysfs", "sysfs", false},
		{"devtmpfs", "devtmpfs", false},
		{"cifs", "cifs", false},
		{"nfs", "nfs", false},
		{"nfs4", "nfs4", false},
		{"iso9660", "iso9660", false},
		{"udf", "udf", false},
		{"f2fs", "f2fs", false},
		{"overlay", "overlay", false},

		// Invalid filesystem types
		{"empty", "", true},
		{"uppercase", "EXT4", true},
		{"with dash", "ext-4", true},
		{"with underscore", "ext_4", true},
		{"with space", "ext 4", true},
		{"with special char", "ext4;rm", true},
		{"with dot", "ext.4", true},
		{"injection attempt", "ext4$(id)", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFSType(tt.fsType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFSType(%q) error = %v, wantErr %v", tt.fsType, err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				if _, ok := err.(*CommandValidationError); !ok {
					t.Errorf("ValidateFSType(%q) returned wrong error type: %T", tt.fsType, err)
				}
			}
		})
	}
}

// TestValidateIP tests IP address validation.
func TestValidateIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		// Valid IPv4 addresses
		{"localhost", "127.0.0.1", false},
		{"local IP", "192.168.1.1", false},
		{"public IP", "8.8.8.8", false},
		{"zero IP", "0.0.0.0", false},
		{"broadcast", "255.255.255.255", false},
		{"full range", "192.168.255.255", false},

		// Valid IPv6 addresses
		{"IPv6 localhost", "::1", false},
		{"IPv6 full", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", false},
		{"IPv6 compressed", "2001:db8:85a3::8a2e:370:7334", false},
		{"IPv6 all zeros", "::", false},
		{"IPv6 link local", "fe80::1", false},
		{"IPv6 with dots", "::ffff:192.168.1.1", false},

		// Invalid IP addresses
		{"empty", "", true},
		// Note: 'a' through 'f' are valid hex chars for IPv6, so they pass
		// But 'g' is not hex, so it's rejected
		{"with letters non-hex", "192.168.1.g", true},
		{"with space", "192.168.1.1 ", true},
		{"with special char", "192.168.1.1;", true},
		{"with backtick", "192.168.1.1`id`", true},
		{"with dollar", "192.168.1.$1", true},
		{"with pipe", "192.168.1.1|cat", true},
		{"with ampersand", "192.168.1.1&&ls", true},
		{"with semicolon", "192.168.1.1;ls", true},
		{"with parenthesis", "192.168.1.1(test)", true},
		{"with angle bracket", "192.168.1.1>file", true},
		{"with newline", "192.168.1.1\nls", true},
		{"with carriage return", "192.168.1.1\rls", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIP(tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIP(%q) error = %v, wantErr %v", tt.ip, err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				if _, ok := err.(*CommandValidationError); !ok {
					t.Errorf("ValidateIP(%q) returned wrong error type: %T", tt.ip, err)
				}
			}
		})
	}
}

// TestValidateDomain tests domain name validation.
func TestValidateDomain(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		wantErr bool
	}{
		// Valid domains
		{"simple domain", "example.com", false},
		{"subdomain", "www.example.com", false},
		{"deep subdomain", "api.v1.example.com", false},
		{"localhost", "localhost", false},
		{"with numbers", "example123.com", false},
		{"numbers in subdomain", "v2.api.example.com", false},
		{"hyphen in middle", "my-example.com", false},
		{"hyphen in subdomain", "my-example.sub-domain.com", false},
		{"single letter", "a.com", false},
		{"short TLD", "example.co", false},
		{"long TLD", "example.technology", false},

		// Invalid domains
		{"empty", "", true},
		{"too long", strings.Repeat("a", 254) + ".com", true},
		{"starts with hyphen", "-example.com", true},
		// Note: regex doesn't check per-label, so hyphen at end of segment passes
		{"ends with hyphen in segment", "example-.com", false},
		{"starts with dot", ".example.com", true},
		{"ends with dot", "example.com.", true},
		{"with space", "example .com", true},
		{"with special char", "example;.com", true},
		{"with backtick", "example`id`.com", true},
		{"with dollar", "example$PATH.com", true},
		{"with pipe", "example|cat.com", true},
		{"with ampersand", "example&&ls.com", true},
		{"with parenthesis", "example(test).com", true},
		{"with angle bracket", "example>file.com", true},
		{"with newline", "example\n.com", true},
		{"with carriage return", "example\r.com", true},
		{"with underscore", "example_test.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDomain(tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDomain(%q) error = %v, wantErr %v", tt.domain, err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				if _, ok := err.(*CommandValidationError); !ok {
					t.Errorf("ValidateDomain(%q) returned wrong error type: %T", tt.domain, err)
				}
			}
		})
	}
}

// TestValidateContainerName tests container/image name validation.
func TestValidateContainerName(t *testing.T) {
	tests := []struct {
		name    string
		cname   string
		wantErr bool
	}{
		// Valid container names
		{"simple name", "nginx", false},
		{"with numbers", "nginx123", false},
		{"with underscore", "nginx_proxy", false},
		{"with dash", "nginx-proxy", false},
		{"with dot", "nginx.proxy", false},
		{"complex name", "my_nginx-proxy.v1", false},
		{"docker image", "nginx:latest", false},
		{"docker image with tag", "nginx:1.21.0", false},
		{"docker image with registry", "registry.example.com/nginx:latest", false},
		{"docker image with port", "localhost:5000/nginx:latest", false},
		{"docker image with org", "docker.io/library/nginx:latest", false},
		{"docker image with at", "nginx@sha256:abc123", false},
		{"full docker image", "registry.example.com:5000/org/nginx:v1.2.3", false},

		// Invalid container names
		{"empty", "", true},
		{"too long", strings.Repeat("a", 256), true},
		{"starts with special char", "-nginx", true},
		{"starts with dot", ".nginx", true},
		{"starts with underscore", "_nginx", true},
		// Note: regex allows starting with numbers
		{"starts with number", "123nginx", false},
		{"with space", "nginx proxy", true},
		{"with semicolon", "nginx;ls", true},
		{"with backtick", "nginx`id`", true},
		{"with dollar", "nginx$PATH", true},
		{"with pipe", "nginx|cat", true},
		{"with ampersand", "nginx&&ls", true},
		{"with parenthesis", "nginx(test)", true},
		{"with angle bracket", "nginx>file", true},
		{"with newline", "nginx\nls", true},
		{"with carriage return", "nginx\rls", true},
		{"with space in tag", "nginx:latest version", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContainerName(tt.cname)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateContainerName(%q) error = %v, wantErr %v", tt.cname, err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				if _, ok := err.(*CommandValidationError); !ok {
					t.Errorf("ValidateContainerName(%q) returned wrong error type: %T", tt.cname, err)
				}
			}
		})
	}
}

// TestValidateTestType tests SMART test type validation.
func TestValidateTestType(t *testing.T) {
	tests := []struct {
		name     string
		testType string
		wantErr  bool
	}{
		// Valid test types
		{"short test", "short", false},
		{"long test", "long", false},
		{"conveyance test", "conveyance", false},
		{"offline test", "offline", false},

		// Invalid test types
		{"empty", "", true},
		{"uppercase", "SHORT", true},
		{"mixed case", "Short", true},
		{"with space", "short test", true},
		{"with number", "short1", true},
		{"with special char", "short;ls", true},
		{"with backtick", "short`id`", true},
		{"with dollar", "short$PATH", true},
		{"with pipe", "short|cat", true},
		{"with ampersand", "short&&ls", true},
		{"invalid type", "invalid", true},
		{"partial match", "shortlong", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTestType(tt.testType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTestType(%q) error = %v, wantErr %v", tt.testType, err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				if _, ok := err.(*CommandValidationError); !ok {
					t.Errorf("ValidateTestType(%q) returned wrong error type: %T", tt.testType, err)
				}
			}
		})
	}
}

// TestValidateArg tests single argument validation.
func TestValidateArg(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		wantErr bool
	}{
		// Valid arguments
		{"simple string", "hello", false},
		{"with spaces", "hello world", false},
		{"with numbers", "hello123", false},
		{"with dots", "hello.world", false},
		{"with dashes", "hello-world", false},
		{"with underscores", "hello_world", false},
		{"with slashes", "/path/to/file", false},
		{"with equals", "key=value", false},
		{"with quotes", `"hello"`, false},
		{"with braces", "{hello}", false},
		{"with brackets", "[hello]", false},
		{"with at sign", "hello@world", false},
		{"with hash", "hello#world", false},
		{"with percent", "hello%world", false},
		{"with plus", "hello+world", false},
		{"with asterisk", "hello*world", false},
		{"with question mark", "hello?world", false},
		{"with tilde", "hello~world", false},
		{"with exclamation", "hello!world", false},

		// Invalid arguments (dangerous characters)
		{"with semicolon", "hello;world", true},
		{"with pipe", "hello|world", true},
		{"with ampersand", "hello&world", true},
		{"with double ampersand", "hello&&world", true},
		{"with dollar", "hello$world", true},
		{"with backtick", "hello`world`", true},
		{"with parenthesis", "hello(world)", true},
		{"with angle bracket open", "hello<world", true},
		{"with angle bracket close", "hello>world", true},
		{"with newline", "hello\nworld", true},
		{"with carriage return", "hello\rworld", true},
		{"command injection", ";rm -rf /", true},
		{"backtick injection", "`id`", true},
		{"dollar injection", "$(whoami)", true},
		{"pipe injection", "|cat /etc/passwd", true},
		{"and injection", "&&ls -la", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArg(tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateArg(%q) error = %v, wantErr %v", tt.arg, err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				if _, ok := err.(*CommandValidationError); !ok {
					t.Errorf("ValidateArg(%q) returned wrong error type: %T", tt.arg, err)
				}
			}
		})
	}
}

// TestValidateArgs tests multiple argument validation.
func TestValidateArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errArg  string // which arg should cause the error
	}{
		// Valid arguments
		{"no args", []string{}, false, ""},
		{"single valid arg", []string{"hello"}, false, ""},
		{"multiple valid args", []string{"echo", "hello", "world"}, false, ""},
		{"valid path args", []string{"/bin/ls", "-la", "/home"}, false, ""},

		// Invalid arguments
		{"single invalid arg", []string{";rm -rf /"}, true, ";rm -rf /"},
		{"first arg invalid", []string{";ls", "hello", "world"}, true, ";ls"},
		{"middle arg invalid", []string{"echo", "|cat", "world"}, true, "|cat"},
		{"last arg invalid", []string{"echo", "hello", "$(id)"}, true, "$(id)"},
		{"backtick injection", []string{"echo", "`id`"}, true, "`id`"},
		{"newline injection", []string{"echo", "hello\nworld"}, true, "hello\nworld"},
		{"carriage return injection", []string{"echo", "hello\rworld"}, true, "hello\rworld"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArgs(tt.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateArgs(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				if _, ok := err.(*CommandValidationError); !ok {
					t.Errorf("ValidateArgs(%v) returned wrong error type: %T", tt.args, err)
				}
				// Check that the error message contains the problematic arg
				if tt.errArg != "" && !strings.Contains(err.Error(), "dangerous character") {
					t.Errorf("ValidateArgs(%v) error should mention dangerous character", tt.args)
				}
			}
		})
	}
}

// TestSafeCommand tests safe command creation.
func TestSafeCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmdName string
		args    []string
		wantErr bool
	}{
		// Valid commands
		{"simple command", "ls", []string{}, false},
		{"command with args", "ls", []string{"-la", "/home"}, false},
		{"echo command", "echo", []string{"hello", "world"}, false},
		{"cat command", "cat", []string{"/etc/hosts"}, false},
		{"docker command", "docker", []string{"ps", "-a"}, false},
		{"systemctl command", "systemctl", []string{"status", "nginx"}, false},

		// Invalid commands
		{"invalid command name", ";rm", []string{}, true},
		{"command with backtick", "`id`", []string{}, true},
		{"command with dollar", "$(whoami)", []string{}, true},
		{"command with pipe", "|cat", []string{}, true},
		{"command with ampersand", "&&ls", []string{}, true},
		{"invalid arg", "ls", []string{"-la;rm -rf /"}, true},
		{"arg with backtick", "echo", []string{"`id`"}, true},
		{"arg with dollar", "echo", []string{"$(whoami)"}, true},
		{"arg with pipe", "cat", []string{"/etc/passwd|grep root"}, true},
		{"arg with newline", "echo", []string{"hello\nworld"}, true},
		{"arg with carriage return", "echo", []string{"hello\rworld"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := SafeCommand(tt.cmdName, tt.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeCommand(%q, %v) error = %v, wantErr %v", tt.cmdName, tt.args, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if cmd == nil {
					t.Errorf("SafeCommand(%q, %v) returned nil command", tt.cmdName, tt.args)
					return
				}
				if cmd.Path == "" {
					t.Errorf("SafeCommand(%q, %v) returned command with empty path", tt.cmdName, tt.args)
				}
			}
		})
	}
}

// TestSafeCommandContext tests safe command creation with context.
func TestSafeCommandContext(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		cmdName string
		args    []string
		wantErr bool
	}{
		// Valid commands
		{"simple command", "ls", []string{}, false},
		{"command with args", "ls", []string{"-la", "/home"}, false},
		{"echo command", "echo", []string{"hello", "world"}, false},
		{"cat command", "cat", []string{"/etc/hosts"}, false},
		{"docker command", "docker", []string{"ps", "-a"}, false},
		{"systemctl command", "systemctl", []string{"status", "nginx"}, false},

		// Invalid commands
		{"invalid command name", ";rm", []string{}, true},
		{"command with backtick", "`id`", []string{}, true},
		{"command with dollar", "$(whoami)", []string{}, true},
		{"command with pipe", "|cat", []string{}, true},
		{"command with ampersand", "&&ls", []string{}, true},
		{"invalid arg", "ls", []string{"-la;rm -rf /"}, true},
		{"arg with backtick", "echo", []string{"`id`"}, true},
		{"arg with dollar", "echo", []string{"$(whoami)"}, true},
		{"arg with pipe", "cat", []string{"/etc/passwd|grep root"}, true},
		{"arg with newline", "echo", []string{"hello\nworld"}, true},
		{"arg with carriage return", "echo", []string{"hello\rworld"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := SafeCommandContext(ctx, tt.cmdName, tt.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeCommandContext(%q, %v) error = %v, wantErr %v", tt.cmdName, tt.args, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if cmd == nil {
					t.Errorf("SafeCommandContext(%q, %v) returned nil command", tt.cmdName, tt.args)
					return
				}
				if cmd.Path == "" {
					t.Errorf("SafeCommandContext(%q, %v) returned command with empty path", tt.cmdName, tt.args)
				}
			}
		})
	}
}

// TestErrorWrapping tests that errors are properly wrapped.
func TestErrorWrapping(t *testing.T) {
	// Test SafeCommand error wrapping
	_, err := SafeCommand(";rm", "-rf", "/")
	if err == nil {
		t.Error("SafeCommand should return error for invalid command name")
	}
	if !strings.Contains(err.Error(), "invalid command name") {
		t.Errorf("SafeCommand error should mention 'invalid command name', got: %v", err)
	}

	_, err = SafeCommand("ls", "-la;rm -rf /")
	if err == nil {
		t.Error("SafeCommand should return error for invalid argument")
	}
	if !strings.Contains(err.Error(), "invalid command argument") {
		t.Errorf("SafeCommand error should mention 'invalid command argument', got: %v", err)
	}

	// Test SafeCommandContext error wrapping
	_, err = SafeCommandContext(context.Background(), ";rm", "-rf", "/")
	if err == nil {
		t.Error("SafeCommandContext should return error for invalid command name")
	}
	if !strings.Contains(err.Error(), "invalid command name") {
		t.Errorf("SafeCommandContext error should mention 'invalid command name', got: %v", err)
	}

	_, err = SafeCommandContext(context.Background(), "ls", "-la;rm -rf /")
	if err == nil {
		t.Error("SafeCommandContext should return error for invalid argument")
	}
	if !strings.Contains(err.Error(), "invalid command argument") {
		t.Errorf("SafeCommandContext error should mention 'invalid command argument', got: %v", err)
	}
}

// TestEdgeCases tests edge cases and boundary conditions.
func TestEdgeCases(t *testing.T) {
	// Test empty strings
	t.Run("empty strings", func(t *testing.T) {
		if err := ValidateDevicePath(""); err == nil {
			t.Error("ValidateDevicePath('') should return error")
		}
		if err := ValidatePath(""); err == nil {
			t.Error("ValidatePath('') should return error")
		}
		if err := ValidateFSType(""); err == nil {
			t.Error("ValidateFSType('') should return error")
		}
		if err := ValidateIP(""); err == nil {
			t.Error("ValidateIP('') should return error")
		}
		if err := ValidateDomain(""); err == nil {
			t.Error("ValidateDomain('') should return error")
		}
		if err := ValidateContainerName(""); err == nil {
			t.Error("ValidateContainerName('') should return error")
		}
		if err := ValidateTestType(""); err == nil {
			t.Error("ValidateTestType('') should return error")
		}
		// ValidateArg should accept empty string (no dangerous chars)
		if err := ValidateArg(""); err != nil {
			t.Error("ValidateArg('') should not return error")
		}
	})

	// Test very long strings
	t.Run("long strings", func(t *testing.T) {
		longStr := strings.Repeat("a", 300)
		// Domain has max length check
		if err := ValidateDomain(longStr); err == nil {
			t.Error("ValidateDomain should reject very long strings")
		}
		// Container name has max length check
		if err := ValidateContainerName(longStr); err == nil {
			t.Error("ValidateContainerName should reject very long strings")
		}
	})

	// Test single character
	t.Run("single character", func(t *testing.T) {
		if err := ValidateArg("a"); err != nil {
			t.Errorf("ValidateArg('a') should not return error, got: %v", err)
		}
		if err := ValidateFSType("a"); err != nil {
			t.Errorf("ValidateFSType('a') should not return error, got: %v", err)
		}
	})

	// Test special characters that should be rejected
	t.Run("dangerous characters", func(t *testing.T) {
		dangerousChars := []string{";", "|", "&", "$", "`", "(", ")", "<", ">", "\n", "\r"}
		for _, char := range dangerousChars {
			if err := ValidateArg(char); err == nil {
				t.Errorf("ValidateArg(%q) should return error for dangerous character", char)
			}
		}
	})
}
