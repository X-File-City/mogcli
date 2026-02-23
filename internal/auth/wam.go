package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// WAMRequest is the JSON payload sent to the mog-wam helper on stdin.
type WAMRequest struct {
	Action    string   `json:"action"` // "login" or "acquire_silent"
	ClientID  string   `json:"client_id"`
	Authority string   `json:"authority"`
	Scopes    []string `json:"scopes"`
	AccountID string   `json:"account_id,omitempty"` // for acquire_silent
	Username  string   `json:"username,omitempty"`   // for acquire_silent
}

// WAMResponse is the JSON payload returned by the mog-wam helper on stdout.
type WAMResponse struct {
	AccessToken   string       `json:"access_token"`
	TokenType     string       `json:"token_type"`
	Scope         string       `json:"scope"`
	ExpiresIn     int          `json:"expires_in"`
	IDToken       string       `json:"id_token"`
	IDTokenClaims *WAMIDClaims `json:"id_token_claims,omitempty"`
	Error         string       `json:"error,omitempty"`
	ErrorDesc     string       `json:"error_description,omitempty"`
}

// WAMIDClaims holds identity claims extracted by the WAM helper.
type WAMIDClaims struct {
	OID               string `json:"oid"`
	Sub               string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	UPN               string `json:"upn"`
	TID               string `json:"tid"`
}

// wamExeName is the filename of the WAM helper binary.
const wamExeName = "mog-wam.exe"

// useWAM reports whether WAM-brokered auth should be used on the current platform.
func useWAM() bool {
	return runtime.GOOS == "windows"
}

// findWAMExe locates the WAM helper binary next to the running executable.
func findWAMExe() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}

	wamPath := filepath.Join(filepath.Dir(self), wamExeName)
	if _, err := os.Stat(wamPath); err != nil {
		return "", fmt.Errorf(
			"WAM authentication helper not found at %s. "+
				"Reinstall mogcli or use app-only mode for headless environments.",
			wamPath,
		)
	}

	return wamPath, nil
}

// invokeWAMExe runs the WAM helper as a subprocess with the given request.
func invokeWAMExe(ctx context.Context, req WAMRequest) (WAMResponse, error) {
	wamExe, err := findWAMExe()
	if err != nil {
		return WAMResponse{}, err
	}

	input, err := json.Marshal(req)
	if err != nil {
		return WAMResponse{}, fmt.Errorf("encode WAM request: %w", err)
	}
	defer secureZero(input)

	cmd := exec.CommandContext(ctx, wamExe)
	cmd.Stdin = bytes.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Try to parse error from stdout first (WAM helper writes JSON errors to stdout).
		var resp WAMResponse
		if jsonErr := json.Unmarshal(stdout.Bytes(), &resp); jsonErr == nil && resp.Error != "" {
			return WAMResponse{}, fmt.Errorf("WAM authentication failed (%s): %s", resp.Error, resp.ErrorDesc)
		}
		return WAMResponse{}, fmt.Errorf("WAM helper process failed: %w", err)
	}

	var resp WAMResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return WAMResponse{}, fmt.Errorf("parse WAM response: %w", err)
	}

	if resp.Error != "" {
		return WAMResponse{}, fmt.Errorf("WAM authentication failed (%s): %s", resp.Error, resp.ErrorDesc)
	}

	return resp, nil
}
