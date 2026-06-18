package rsync

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Cmd struct {
	Port            int
	NoChown         bool
	NonRoot         bool
	Delete          bool
	DeleteAfter     bool
	ExcludeSnapshot bool
	SrcUseSSH       bool
	DestUseSSH      bool
	Command         string
	SrcSSHUser      string
	SrcSSHHost      string
	SrcPath         string
	DestSSHUser     string
	DestSSHHost     string
	DestPath        string
	Compress        bool
	ExtraArgs       string
}

// sshArgsStr returns the quoted "-e" ssh command string used for remote transfers.
func (c *Cmd) sshArgsStr() string {
	sshArgs := []string{
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=5",
		// ServerAliveInterval/CountMax prevent intermediate load balancers and proxies
		// from dropping idle SSH connections during long file-list-building phases.
		"-o", "ServerAliveInterval=10",
		"-o", "ServerAliveCountMax=3",
	}
	if c.Port != 0 {
		sshArgs = append(sshArgs, "-p", strconv.Itoa(c.Port))
	}

	return fmt.Sprintf("\"%s\"", strings.Join(sshArgs, " "))
}

// rsyncArgsStr returns the common rsync flags shared by single and batch
// transfers. Keeping a single source of truth ensures flags such as
// ExtraArgs apply consistently to every rsync invocation in a batch.
func (c *Cmd) rsyncArgsStr() string {
	rsyncArgs := []string{
		"-av", "--info=progress2,misc0,flist0",
		"--no-inc-recursive", "-e", c.sshArgsStr(),
	}

	if c.Compress {
		rsyncArgs = append(rsyncArgs, "-z")
	}

	if c.NoChown || c.NonRoot {
		rsyncArgs = append(rsyncArgs, "--no-o", "--no-g")
	}

	if c.NonRoot {
		rsyncArgs = append(rsyncArgs, "--omit-dir-times")
	}

	if c.Delete {
		rsyncArgs = append(rsyncArgs, "--delete")
	}

	if c.DeleteAfter {
		rsyncArgs = append(rsyncArgs, "--delete-after")
	}

	if c.ExcludeSnapshot {
		rsyncArgs = append(rsyncArgs, "--exclude='.snapshot'")
	}

	if c.ExtraArgs != "" {
		rsyncArgs = append(rsyncArgs, c.ExtraArgs)
	}

	return strings.Join(rsyncArgs, " ")
}

func (c *Cmd) Build() (string, error) {
	if c.SrcUseSSH && c.DestUseSSH {
		return "", errors.New("cannot use ssh on both source and destination")
	}

	cmd := "rsync"
	if c.Command != "" {
		cmd = c.Command
	}

	src := c.buildSrc()
	dest := c.buildDest()

	return fmt.Sprintf("%s %s %s %s", cmd, c.rsyncArgsStr(), src, dest), nil
}

func (c *Cmd) buildSrc() string {
	var src strings.Builder

	if c.SrcUseSSH {
		sshDestUser := "root"
		if c.SrcSSHUser != "" {
			sshDestUser = c.SrcSSHUser
		}

		fmt.Fprintf(&src, "%s@%s:", sshDestUser, c.SrcSSHHost)
	}

	src.WriteString(c.SrcPath)

	return src.String()
}

func (c *Cmd) buildDest() string {
	var dest strings.Builder

	if c.DestUseSSH {
		sshDestUser := "root"
		if c.DestSSHUser != "" {
			sshDestUser = c.DestSSHUser
		}

		fmt.Fprintf(&dest, "%s@%s:", sshDestUser, c.DestSSHHost)
	}

	dest.WriteString(c.DestPath)

	return dest.String()
}

// BatchEntry describes a single src→dest mapping within a batch rsync.
type BatchEntry struct {
	SrcPath  string
	DestPath string
}

// BuildBatch generates a compound shell command that runs rsync for
// each (SrcPath, DestPath) pair in entries, in order.
// All entries share the same SSH host + settings from the receiver Cmd.
func (c *Cmd) BuildBatch(entries []BatchEntry) (string, error) {
	if len(entries) == 0 {
		return "", errors.New("no batch entries provided")
	}

	if c.SrcUseSSH && c.DestUseSSH {
		return "", errors.New("cannot use ssh on both source and destination")
	}

	cmd := "rsync"
	if c.Command != "" {
		cmd = c.Command
	}

	// Share the exact same flag set as a single Build() so that ExtraArgs
	// (e.g. --iconv), --delete-after, snapshot excludes and SSH keepalives
	// apply to every entry in the batch, not just the last one.
	rsyncArgsStr := c.rsyncArgsStr()

	// Build individual commands.
	var parts []string
	for _, e := range entries {
		src := c.buildSrcForPath(e.SrcPath)
		parts = append(parts, fmt.Sprintf("%s %s %s %s", cmd, rsyncArgsStr, src, e.DestPath))
	}

	return strings.Join(parts, " && "), nil
}

// buildSrcForPath constructs the source argument for a given path,
// using the SSH host from the receiver when SrcUseSSH is set.
func (c *Cmd) buildSrcForPath(path string) string {
	if !c.SrcUseSSH {
		return path
	}

	sshUser := "root"
	if c.SrcSSHUser != "" {
		sshUser = c.SrcSSHUser
	}

	return fmt.Sprintf("%s@%s:%s", sshUser, c.SrcSSHHost, path)
}
